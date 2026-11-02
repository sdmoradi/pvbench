package benchmark

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
)

const (
	// DefaultBenchmarkImage is the FIO image used by benchmark Jobs.
	DefaultBenchmarkImage = "docker.io/sdmoradi/fio-bench:latest"

	managedByLabel = "pvbench.sdmoradi.dev/managed-by"
	managedByValue = "pvbench"
	benchmarkLabel = "app.kubernetes.io/component"
	benchmarkValue = "benchmark"
)

// CreatePVC creates a disposable filesystem PVC for one benchmark run.
// The caller is responsible for deleting this PVC after collecting results.
func CreatePVC(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	storageClass string,
	size string,
) (*corev1.PersistentVolumeClaim, error) {
	if strings.TrimSpace(namespace) == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if strings.TrimSpace(storageClass) == "" {
		return nil, fmt.Errorf("storage class name is required")
	}

	quantity, err := resource.ParseQuantity(size)
	if err != nil {
		return nil, fmt.Errorf("invalid PVC size %q: %w", size, err)
	}
	if quantity.Sign() <= 0 {
		return nil, fmt.Errorf("PVC size must be greater than zero")
	}

	scName := storageClass
	volumeMode := corev1.PersistentVolumeFilesystem

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pvbench-",
			Namespace:    namespace,
			Labels: map[string]string{
				managedByLabel: managedByValue,
				benchmarkLabel: benchmarkValue,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			VolumeMode:       &volumeMode,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: quantity,
				},
			},
		},
	}

	created, err := client.CoreV1().
		PersistentVolumeClaims(namespace).
		Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsForbidden(err) {
			return nil, fmt.Errorf("not allowed to create benchmark PVC in namespace %q: %w", namespace, err)
		}
		return nil, fmt.Errorf("create benchmark PVC in namespace %q: %w", namespace, err)
	}

	return created, nil
}
