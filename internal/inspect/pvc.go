package inspect

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type PVCInfo struct {
	Name          string
	Namespace     string
	Status        string
	Capacity      string
	AccessModes   []string
	VolumeMode    string
	PVName        string
	StorageClass  string
	Provisioner   string
	ReclaimPolicy string
}

func PVC(
	ctx context.Context,
	client kubernetes.Interface,
	namespace string,
	name string,
) (*PVCInfo, error) {
	pvc, err := client.CoreV1().
		PersistentVolumeClaims(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf(
				"PVC %q not found in namespace %q",
				name,
				namespace,
			)
		}

		return nil, fmt.Errorf("get PVC: %w", err)
	}

	info := &PVCInfo{
		Name:      pvc.Name,
		Namespace: pvc.Namespace,
		Status:    string(pvc.Status.Phase),
		PVName:    pvc.Spec.VolumeName,
	}

	if pvc.Spec.VolumeMode != nil {
		info.VolumeMode = string(*pvc.Spec.VolumeMode)
	} else {
		info.VolumeMode = string(corev1.PersistentVolumeFilesystem)
	}

	for _, mode := range pvc.Spec.AccessModes {
		info.AccessModes = append(info.AccessModes, string(mode))
	}

	if capacity, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
		info.Capacity = capacity.String()
	}

	if pvc.Spec.StorageClassName != nil {
		info.StorageClass = *pvc.Spec.StorageClassName
	}

	if info.PVName == "" {
		return info, nil
	}

	pv, err := client.CoreV1().
		PersistentVolumes().
		Get(ctx, info.PVName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get PV %q: %w", info.PVName, err)
	}

	info.ReclaimPolicy = string(pv.Spec.PersistentVolumeReclaimPolicy)

	// Use the PV's StorageClass reference when available.
	if pv.Spec.StorageClassName != "" {
		info.StorageClass = pv.Spec.StorageClassName
	}

	if info.StorageClass == "" {
		return info, nil
	}

	sc, err := client.StorageV1().
		StorageClasses().
		Get(ctx, info.StorageClass, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			info.Provisioner = "StorageClass not found"
			return info, nil
		}

		return nil, fmt.Errorf("get StorageClass %q: %w", info.StorageClass, err)
	}

	info.Provisioner = sc.Provisioner

	return info, nil
}
