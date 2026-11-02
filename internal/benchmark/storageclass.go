package benchmark

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/client-go/kubernetes"
)

// GetStorageClass retrieves a StorageClass by its exact name.
// It never silently falls back to the cluster's default StorageClass.
func GetStorageClass(ctx context.Context, client kubernetes.Interface, name string) (*storagev1.StorageClass, error) {
	if name == "" {
		return nil, fmt.Errorf("storage class name is required")
	}

	sc, err := client.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("StorageClass %q not found", name)
		}
		return nil, fmt.Errorf("get StorageClass %q: %w", name, err)
	}
	return sc, nil
}
