package reconcile

import (
	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func backupOwnerReference(backup *minecraftv1alpha1.MinecraftBackup) metav1.OwnerReference {
	return *metav1.NewControllerRef(backup, minecraftv1alpha1.GroupVersion.WithKind("MinecraftBackup"))
}

func ownerReference(server *minecraftv1alpha1.MinecraftServer) metav1.OwnerReference {
	return *metav1.NewControllerRef(server, minecraftv1alpha1.GroupVersion.WithKind("MinecraftServer"))
}

// hasCorrectOwnerReference verifies that the given object has the correct owner reference set on it
func hasCorrectOwnerReference(server *minecraftv1alpha1.MinecraftServer, actual metav1.Object) bool {
	expected := ownerReference(server)
	for _, ow := range actual.GetOwnerReferences() {
		if ow.APIVersion == expected.APIVersion &&
			ow.Name == expected.Name &&
			ow.Kind == expected.Kind {
			return true
		}
	}
	return false
}

func podLabels(server *minecraftv1alpha1.MinecraftServer) map[string]string {
	return map[string]string{
		"app":       "minecraft",
		"minecraft": server.Name,
	}
}
