package minecraftbackup

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
)

func backupOwnerReference(backup *minecraftv1alpha1.MinecraftBackup) metav1.OwnerReference {
	return *metav1.NewControllerRef(backup, minecraftv1alpha1.GroupVersion.WithKind("MinecraftBackup"))
}
