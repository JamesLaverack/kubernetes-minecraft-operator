/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	minecraftv1alpha1 "github.com/jameslaverack/minecraft-operator/api/v1alpha1"
	"github.com/jameslaverack/minecraft-operator/pkg/reconcile"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MinecraftServerReconciler reconciles a MinecraftServer object
type MinecraftServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Logger *zap.SugaredLogger
}

//+kubebuilder:rbac:groups=minecraft.jameslaverack.com,resources=minecraftservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=minecraft.jameslaverack.com,resources=minecraftservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=minecraft.jameslaverack.com,resources=minecraftservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *MinecraftServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.With("name", req.Name, "namespace", req.Namespace)

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
	configMap, action, err := reconcile.ReconcileConfigMap(ctx, logger, r, &server)
	if err != nil {
		return ctrl.Result{}, err
	}
	if action != nil {
		// We've been given an action, so execute it and return the result.
		logger.Debug("Given action for reconciling Config Map")
		return action(ctx, logger, r)
	}

	_, action, err = reconcile.ReconcilePod(ctx, logger, r, &server, configMap)
	if err != nil {
		return ctrl.Result{}, err
	}
	if action != nil {
		// We've been given an action, so execute it and return the result.
		logger.Debug("Given action for reconciling Pod")
		return action(ctx, logger, r)
	}

	// TODO Detect if we need to restart the pod due to a config change. At the moment a Vanilla Tweaks change, for
	//      example, will result in the configmap being updated but the server won't update. We need to detect if the
	//      vanilla tweaks that are expected are *actually* loaded and if they're not then restart the pod

	// TODO Handle allowlist and oplist changes. At the moment it won't be updated at all but it also doesn't need a
	//      Pod restart if we can use RCONS or something.

	_, action, err = reconcile.ReconcileService(ctx, logger, r, &server)
	if err != nil {
		return ctrl.Result{}, err
	}
	if action != nil {
		// We've been given an action, so execute it and return the result.
		logger.Debug("Given action for reconciling Service")
		return action(ctx, logger, r)
	}

	if server.Spec.Monitoring.Enabled {
		_, action, err = reconcile.ReconcilePodMonitorExists(ctx, logger, r, &server)
	} else {
		action, err = reconcile.ReconcilePodMonitorNotExists(ctx, logger, r, &server)
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	if action != nil {
		// We've been given an action, so execute it and return the result.
		logger.Debug("Given action for reconciling pod monitor")
		return action(ctx, logger, r)
	}

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
		Owns(&monitoringv1.PodMonitor{}).
		Complete(r)
}
