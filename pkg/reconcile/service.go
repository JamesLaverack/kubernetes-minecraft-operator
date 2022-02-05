package reconcile

import (
	"context"
	"github.com/go-logr/logr"
	minecraftv1alpha1 "github.com/jameslaverack/minecraft-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReconcileService(ctx context.Context, logger logr.Logger, reader client.Reader, server *minecraftv1alpha1.MinecraftServer) (*corev1.Service, ReconcileAction, error) {
	expectedService := serviceForServer(server)

	var actualService corev1.Service
	err := reader.Get(ctx, client.ObjectKeyFromObject(&expectedService), &actualService)
	if apierrors.IsNotFound(err) {
		// Pretty simple, just create it
		return &expectedService,
			func(ctx context.Context, logger logr.Logger, writer client.Writer) (ctrl.Result, error) {
				logger.Info("Creating Minecraft Service")
				return ctrl.Result{}, writer.Create(ctx, &expectedService)
			},
			nil
	}
	if err != nil {
		return nil, nil, err
	}

	// Check service for integrity
	if !hasCorrectOwnerReference(server, &actualService) {
		// Set the right owner reference. Adding it to any existing ones.
		actualService.OwnerReferences = append(actualService.OwnerReferences, ownerReference(server))
		return &actualService,
			func(ctx context.Context, logger logr.Logger, writer client.Writer) (ctrl.Result, error) {
				logger.Info("Setting owner reference on Service")
				return ctrl.Result{}, writer.Update(ctx, &actualService)
			},
			nil
	}

	if reflect.DeepEqual(expectedService.Spec, actualService.Spec) {
		return &actualService,
			func(ctx context.Context, logger logr.Logger, writer client.Writer) (ctrl.Result, error) {
				logger.Info("Service spec is incorrect, patching")
				return ctrl.Result{}, writer.Update(ctx, &actualService)
			},
			nil
	}

	logger.V(0).Info("Service all okay")
	return &actualService, nil, nil
}

func serviceForServer(server *minecraftv1alpha1.MinecraftServer) corev1.Service {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
		},
		Spec: corev1.ServiceSpec{
			// TODO Make configurable
			Type:     corev1.ServiceTypeLoadBalancer,
			Selector: podLabels(server),
			Ports: []corev1.ServicePort{
				{
					Name: "minecraft",
					Port: 25565,
				},
			},
		},
	}

	if server.Spec.ExternalServiceIP != "" {
		service.Spec.ExternalIPs = []string{server.Spec.ExternalServiceIP}
	}

	return service
}
