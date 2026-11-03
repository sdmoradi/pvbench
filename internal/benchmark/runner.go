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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

// Options configures one disposable benchmark run.
type Options struct {
	Namespace     string
	StorageClass  string
	Size          string
	Profile       string
	Timeout       time.Duration
	KeepResources bool
}

// Run executes FIO, prints a parsed summary, and removes resources it created.
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
	if qty.Cmp(resource.MustParse("1Gi")) < 0 {
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
		fmt.Fprintln(output, "\nCleanup")
		fmt.Fprintln(output, "-------")
		if opts.KeepResources {
			fmt.Fprintf(output, "Temporary PVC and Job retained: %s", pvc.Name)
			if job != nil {
				fmt.Fprintf(output, ", %s", job.Name)
			}
			fmt.Fprintln(output)
			return
		}

		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var cleanupErr error
		if job != nil {
			if err := client.BatchV1().Jobs(opts.Namespace).Delete(cleanupCtx, job.Name, metav1.DeleteOptions{PropagationPolicy: ptrPolicy(metav1.DeletePropagationBackground)}); err != nil && !apierrors.IsNotFound(err) {
				cleanupErr = joinError(cleanupErr, fmt.Errorf("delete benchmark Job %q: %w", job.Name, err))
			}
		}
		if err := client.CoreV1().PersistentVolumeClaims(opts.Namespace).Delete(cleanupCtx, pvc.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			cleanupErr = joinError(cleanupErr, fmt.Errorf("delete benchmark PVC %q: %w", pvc.Name, err))
		}
		if cleanupErr != nil {
			fmt.Fprintf(output, "FAILED: %v\n", cleanupErr)
			retErr = joinError(retErr, cleanupErr)
			return
		}
		fmt.Fprintln(output, "Temporary PVC and Job deleted.")
	}()

	job, err = CreateJob(ctx, client, opts.Namespace, pvc.Name, opts.Profile)
	if err != nil {
		return err
	}

	runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()
	waitErr := wait.PollUntilContextCancel(runCtx, 2*time.Second, true, func(ctx context.Context) (bool, error) {
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
	if waitErr != nil && runCtx.Err() != nil {
		waitErr = fmt.Errorf("benchmark timed out after %s", opts.Timeout)
	}

	logCtx, logCancel := context.WithTimeout(context.Background(), 15*time.Second)
	logs, logsErr := jobLogs(logCtx, client, opts.Namespace, job.Name)
	logCancel()

	results := parseWorkloadResults(logs, opts.Profile)
	executionErr := waitErr
	if logsErr != nil {
		executionErr = joinError(executionErr, logsErr)
	}
	renderSummary(output, opts.Namespace, opts.StorageClass, opts.Size, opts.Profile, results, executionErr)

	if executionErr != nil {
		return executionErr
	}
	for _, result := range results {
		if !result.Success {
			return fmt.Errorf("one or more FIO workloads did not produce valid results")
		}
	}
	return nil
}

func jobLogs(ctx context.Context, client kubernetes.Interface, namespace, jobName string) (string, error) {
	selector := labels.Set{"batch.kubernetes.io/job-name": jobName}.AsSelector().String()
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return "", fmt.Errorf("list benchmark Job pods: %w", err)
	}
	if len(pods.Items) == 0 {
		selector = labels.Set{"job-name": jobName}.AsSelector().String()
		pods, err = client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return "", fmt.Errorf("find benchmark Job pod: %w", err)
		}
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no Pod found for benchmark Job %q", jobName)
	}
	pod := pods.Items[0]
	stream, err := client.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Container: "fio"}).Stream(ctx)
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

func ptrPolicy(policy metav1.DeletionPropagation) *metav1.DeletionPropagation {
	return &policy
}

func joinError(first, second error) error {
	if first == nil {
		return second
	}
	return fmt.Errorf("%v; additionally: %w", first, second)
}
