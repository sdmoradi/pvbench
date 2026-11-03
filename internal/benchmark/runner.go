package benchmark

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
)

// Options configures one disposable benchmark run.
type Options struct {
	Namespace   string
	StorageClass string
	Size         string
	Profile      string
	Timeout      time.Duration
	KeepResources bool
}

// Run creates a temporary PVC and FIO Job, waits for completion, prints the Job
// logs to the supplied writer, and removes resources it created unless retained.
func Run(ctx context.Context, client kubernetes.Interface, opts Options, output io.Writer) (retErr error) {
	if strings.TrimSpace(opts.Namespace) == "" {
		return fmt.Errorf("namespace is required")
	}
	if strings.TrimSpace(opts.StorageClass) == "" {
		return fmt.Errorf("storage class is required")
	}
	if opts.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	qty, err := resource.ParseQuantity(opts.Size)
	if err != nil || qty.Sign() <= 0 {
		return fmt.Errorf("invalid PVC size %q: must be a positive Kubernetes storage quantity", opts.Size)
	}
	minimum := resource.MustParse("1Gi")
	if qty.Cmp(minimum) < 0 {
		return fmt.Errorf("PVC size must be at least 1Gi for the current FIO test file")
	}
	if _, err := fioScript(opts.Profile); err != nil {
		return err
	}
	if _, err := GetStorageClass(ctx, client, opts.StorageClass); err != nil {
		return err
	}

	pvc, err := CreatePVC(ctx, client, opts.Namespace, opts.StorageClass, opts.Size)
	if err != nil {
		return err
	}
	var job *batchv1.Job
	defer func() {
		if opts.KeepResources {
			fmt.Fprintf(output, "Keeping benchmark resources: PVC %s", pvc.Name)
			if job != nil {
				fmt.Fprintf(output, ", Job %s", job.Name)
			}
			fmt.Fprintln(output)
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if job != nil {
			if err := client.BatchV1().Jobs(opts.Namespace).Delete(cleanupCtx, job.Name, metav1.DeleteOptions{PropagationPolicy: ptrPolicy(metav1.DeletePropagationBackground)}); err != nil && !apierrors.IsNotFound(err) {
				retErr = joinError(retErr, fmt.Errorf("delete benchmark Job %q: %w", job.Name, err))
			}
		}
		if err := client.CoreV1().PersistentVolumeClaims(opts.Namespace).Delete(cleanupCtx, pvc.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			retErr = joinError(retErr, fmt.Errorf("delete benchmark PVC %q: %w", pvc.Name, err))
		}
	}()

	job, err = CreateJob(ctx, client, opts.Namespace, pvc.Name, opts.Profile)
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "Benchmark started: namespace=%s storageClass=%s pvc=%s job=%s profile=%s size=%s\n",
		opts.Namespace, opts.StorageClass, pvc.Name, job.Name, opts.Profile, opts.Size)

	runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	err = wait.PollUntilContextCancel(runCtx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
		current, err := client.BatchV1().Jobs(opts.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, condition := range current.Status.Conditions {
			if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
				return false, fmt.Errorf("benchmark Job failed: %s: %s", condition.Reason, condition.Message)
			}
			if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		if runCtx.Err() != nil {
			return fmt.Errorf("benchmark timed out after %s", opts.Timeout)
		}
		return fmt.Errorf("wait for benchmark Job: %w", err)
	}

	logs, err := jobLogs(ctx, client, opts.Namespace, job.Name)
	if err != nil {
		return err
	}
	fmt.Fprintln(output, "\nFIO OUTPUT")
	fmt.Fprintln(output, logs)
	return nil
}

func jobLogs(ctx context.Context, client kubernetes.Interface, namespace, jobName string) (string, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set{"batch.kubernetes.io/job-name": jobName}.AsSelector().String(),
	})
	if err != nil {
		return "", fmt.Errorf("list benchmark Job pods: %w", err)
	}
	if len(pods.Items) == 0 {
		// Older clusters may use the legacy job-name label.
		pods, err = client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("metadata.name", "").String(),
			LabelSelector: labels.Set{"job-name": jobName}.AsSelector().String(),
		})
		if err != nil {
			return "", fmt.Errorf("find benchmark Job pod: %w", err)
		}
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no Pod found for benchmark Job %q", jobName)
	}
	pod := pods.Items[0]
	req := client.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: "fio"})
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("read benchmark logs from Pod %q: %w", pod.Name, err)
	}
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("read benchmark log stream: %w", err)
	}
	return string(data), nil
}

func ptrPolicy(policy metav1.DeletionPropagation) *metav1.DeletionPropagation { return &policy }

func joinError(first, second error) error {
	if first == nil { return second }
	return fmt.Errorf("%v; additionally: %w", first, second)
}
