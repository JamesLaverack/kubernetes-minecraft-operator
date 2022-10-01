package minecraftserver

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
)

func serverOwnerReference(server *minecraftv1alpha1.MinecraftServer) metav1.OwnerReference {
	return *metav1.NewControllerRef(server, minecraftv1alpha1.GroupVersion.WithKind("MinecraftServer"))
}

// hasCorrectOwnerReference verifies that the given object has the correct owner reference set on it
func hasCorrectOwnerReference(server *minecraftv1alpha1.MinecraftServer, actual metav1.Object) bool {
	expected := serverOwnerReference(server)
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
