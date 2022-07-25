package controllers

import (
	"context"
	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/reconcile"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// MinecraftServerReconciler reconciles a MinecraftServer object
type MinecraftServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *MinecraftServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.Log.
		WithName("controller").
		WithName("minecraftserver").
		WithValues(
			"name", req.Name,
			"namespace", req.Namespace)

	logger.Info("beginning reconciliation")
	// Go back to the API server with a get to find the full definition of the MinecraftServer object (we're only given
	// the name and namespace at this point). We also might fail to find it, as we might have been triggered to
	// reconcile because the object was deleted. In this case we don't need to do any cleanup, as we set the owner
	// references on every other object we create so the API server's normal cascading delete behaviour will clean up
	// everything.
	var server minecraftv1alpha1.MinecraftServer
	if err := r.Get(ctx, req.NamespacedName, &server); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// We'll now create each resource we need. In general we'll "reconcile" each resource in turn. If there's work to be
	// done we'll do it an exit instantly. This is because this function is triggered on changes to owned resources, so
	// the act of creating or modifying an owned resource will cause this function to be called again anyway.

	// Config map. This holds configuration files for Minecraft, along with things like the vanilla tweaks config JSON.
	configMap, action, err := reconcile.ReconcileConfigMap(ctx, logger.WithName("configmap"), r, &server)
	if err != nil {
		return ctrl.Result{}, err
	}
	if action != nil {
		// We've been given an action, so execute it and return the result.
		logger.V(1).Info("Given action for reconciling Config Map")
		return action(ctx, logger.WithName("configmap").WithName("action"), r)
	}

	_, action, err = reconcile.ReconcilePod(ctx, logger.WithName("pod"), r, &server, configMap)
	if err != nil {
		return ctrl.Result{}, err
	}
	if action != nil {
		// We've been given an action, so execute it and return the result.
		logger.V(1).Info("Given action for reconciling Pod")
		return action(ctx, logger.WithName("pod").WithName("action"), r)
	}

	// TODO Detect if we need to restart the pod due to a config change. At the moment a Vanilla Tweaks change, for
	//      example, will result in the configmap being updated but the server won't update. We need to detect if the
	//      vanilla tweaks that are expected are *actually* loaded and if they're not then restart the pod

	// TODO Handle allowlist and oplist changes. At the moment it won't be updated at all but it also doesn't need a
	//      Pod restart if we can use RCONS or something.

	_, action, err = reconcile.ReconcileService(ctx, logger.WithName("service"), r, &server)
	if err != nil {
		return ctrl.Result{}, err
	}
	if action != nil {
		// We've been given an action, so execute it and return the result.
		logger.V(1).Info("Given action for reconciling Service")
		return action(ctx, logger.WithName("service").WithName("action"), r)
	}

	//if server.Spec.Monitoring.Enabled {
	//	_, action, err = reconcile.ReconcilePodMonitorExists(ctx, logger, r, &server)
	//} else {
	//	action, err = reconcile.ReconcilePodMonitorNotExists(ctx, logger, r, &server)
	//}
	//if err != nil {
	//	return ctrl.Result{}, err
	//}
	//if action != nil {
	//	// We've been given an action, so execute it and return the result.
	//	logger.Debug("Given action for reconciling pod monitor")
	//	return action(ctx, logger, r)
	//}

	// All good, return
	logger.Info("All good")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinecraftServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&minecraftv1alpha1.MinecraftServer{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		//Owns(&monitoringv1.PodMonitor{}).
		Complete(r)
}
