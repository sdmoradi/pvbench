package benchmark

import (
	"context"
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
)

const (
	benchmarkDataPath = "/pvbench-data/test.bin"
	benchmarkMount    = "/pvbench-data"
)

// CreateJob creates a Kubernetes Job that runs FIO against the temporary PVC.
// The PVC must already exist. The Job does not delete the PVC; cleanup is owned
// by the benchmark runner after logs have been collected.
func CreateJob(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	pvcName string,
	profile string,
) (*batchv1.Job, error) {
	if strings.TrimSpace(namespace) == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if strings.TrimSpace(pvcName) == "" {
		return nil, fmt.Errorf("PVC name is required")
	}

	script, err := fioScript(profile)
	if err != nil {
		return nil, err
	}

	backoffLimit := int32(0)
	ttlSecondsAfterFinished := int32(300)
	labels := map[string]string{
		managedByLabel: managedByValue,
		benchmarkLabel: benchmarkValue,
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pvbench-",
			Namespace:    namespace,
			Labels:       labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "fio",
							Image:   DefaultBenchmarkImage,
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{script},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "benchmark-data", MountPath: benchmarkMount},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2"),
									corev1.ResourceMemory: resource.MustParse("1Gi"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "benchmark-data",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcName,
								},
							},
						},
					},
				},
			},
		},
	}

	created, err := client.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create benchmark Job in namespace %q: %w", namespace, err)
	}
	return created, nil
}

// fioScript returns a shell script for the selected workload profile.
// The initial fill ensures read workloads have a real file to read.
func fioScript(profile string) (string, error) {
	var workloads []string
	switch profile {
	case "read":
		workloads = []string{"seq-read", "rand-read"}
	case "write":
		workloads = []string{"seq-write", "rand-write"}
	case "all":
		workloads = []string{"seq-read", "seq-write", "rand-read", "rand-write"}
	default:
		return "", fmt.Errorf("invalid benchmark profile %q (expected read, write, or all)", profile)
	}

	var b strings.Builder
	b.WriteString("set -eu\n")
	b.WriteString("FIO=/usr/bin/fio\n")
	b.WriteString("FILE=" + benchmarkDataPath + "\n")
	b.WriteString("SIZE=1G\n")
	b.WriteString("echo 'PVBench: preparing benchmark file'\n")
	b.WriteString("$FIO --name=prepare --filename=\"$FILE\" --size=$SIZE --rw=write --bs=1M --direct=1 --ioengine=libaio --iodepth=16 --numjobs=1 --group_reporting --output-format=json\n")

	for _, workload := range workloads {
		rw := "read"
		bs := "1M"
		switch workload {
		case "seq-write":
			rw = "write"
		case "rand-read":
			rw = "randread"
			bs = "4k"
		case "rand-write":
			rw = "randwrite"
			bs = "4k"
		}

		fmt.Fprintf(&b, "echo 'PVBench:RESULT:%s:BEGIN'\n", workload)
		fmt.Fprintf(&b, "$FIO --name=%s --filename=\"$FILE\" --size=$SIZE --rw=%s --bs=%s --direct=1 --ioengine=libaio --iodepth=16 --numjobs=1 --runtime=30 --time_based --group_reporting --output-format=json\n", workload, rw, bs)
		fmt.Fprintf(&b, "echo 'PVBench:RESULT:%s:END'\n", workload)
	}
	return b.String(), nil
}
