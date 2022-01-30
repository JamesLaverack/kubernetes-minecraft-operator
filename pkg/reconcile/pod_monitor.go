package reconcile

import (
	"context"
	minecraftv1alpha1 "github.com/jameslaverack/minecraft-operator/api/v1alpha1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReconcilePodMonitorNotExists(ctx context.Context, logger *zap.SugaredLogger, reader client.Reader, server *minecraftv1alpha1.MinecraftServer) (ReconcileAction, error) {
	expectedPodMonitor := podMonitor(server)
	var actualPodMonitor monitoringv1.PodMonitor
	err := reader.Get(ctx, client.ObjectKeyFromObject(&expectedPodMonitor), &actualPodMonitor)
	if err != nil {
		// All good.
		return nil, client.IgnoreNotFound(err)
	}
	return func(ctx context.Context, logger *zap.SugaredLogger, writer client.Writer) (ctrl.Result, error) {
			logger.Info("Deleting Pod Monitor")
			return ctrl.Result{}, writer.Delete(ctx, &expectedPodMonitor)
		},
		nil
}

func ReconcilePodMonitorExists(ctx context.Context, logger *zap.SugaredLogger, reader client.Reader, server *minecraftv1alpha1.MinecraftServer) (*monitoringv1.PodMonitor, ReconcileAction, error) {
	expectedPodMonitor := podMonitor(server)

	var actualPodMonitor monitoringv1.PodMonitor
	err := reader.Get(ctx, client.ObjectKeyFromObject(&expectedPodMonitor), &actualPodMonitor)
	if apierrors.IsNotFound(err) {
		// Pretty simple, just create it
		return &expectedPodMonitor,
			func(ctx context.Context, logger *zap.SugaredLogger, writer client.Writer) (ctrl.Result, error) {
				logger.Info("Creating Pod Monitor")
				return ctrl.Result{}, writer.Create(ctx, &expectedPodMonitor)
			},
			nil
	}
	if err != nil {
		return nil, nil, err
	}

	// Check pod monitor for integrity
	if !hasCorrectOwnerReference(server, &actualPodMonitor) {
		// Set the right owner reference. Adding it to any existing ones.
		actualPodMonitor.OwnerReferences = append(actualPodMonitor.OwnerReferences, ownerReference(server))
		return &actualPodMonitor,
			func(ctx context.Context, logger *zap.SugaredLogger, writer client.Writer) (ctrl.Result, error) {
				logger.Info("Setting owner reference on pod monitor")
				return ctrl.Result{}, writer.Patch(ctx, &actualPodMonitor, client.MergeFrom(&actualPodMonitor))
			},
			nil
	}

	if reflect.DeepEqual(expectedPodMonitor.Spec, actualPodMonitor.Spec) {
		return &actualPodMonitor,
			func(ctx context.Context, logger *zap.SugaredLogger, writer client.Writer) (ctrl.Result, error) {
				logger.Info("pod monitor spec is incorrect, patching")
				return ctrl.Result{}, writer.Patch(ctx, &actualPodMonitor, client.MergeFrom(&actualPodMonitor))
			},
			nil
	}

	logger.Debug("pod monitor all okay")
	return &actualPodMonitor, nil, nil
}

func podMonitor(server *minecraftv1alpha1.MinecraftServer) monitoringv1.PodMonitor {
	return monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:            server.Name,
			Namespace:       server.Namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(server, minecraftv1alpha1.GroupVersion.WithKind("MinecraftServer"))},
		},
		Spec: monitoringv1.PodMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: podLabels(server),
			},
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
				{
					Port: "metrics",
				},
			},
		},
	}
}
