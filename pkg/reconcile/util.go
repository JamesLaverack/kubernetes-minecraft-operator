package reconcile

import (
	"context"
	minecraftv1alpha1 "github.com/jameslaverack/minecraft-operator/api/v1alpha1"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconcileAction represents a single, simple action to take. The action itself returns a result and an error
type ReconcileAction func(ctx context.Context, logger *zap.SugaredLogger, writer client.Writer) (ctrl.Result, error)

// hasCorrectLabels verifies that the actual labels on a object include the expected labels. We're perfectly okay with
// *extra* labels (which may be added by webhooks or whatever), but we want to verify that we have all of the labels we
// expect and that they're the correct values.
func hasCorrectLabels(expected, actual metav1.Object) bool {
	for k, v := range expected.GetLabels() {
		if actual.GetLabels()[k] != v {
			return false
		}
	}
	return true
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
			ow.Kind == expected.Name {
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
