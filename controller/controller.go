package controller

import (
	"context"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/logutil"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/reconcile"
)

// MinecraftServerReconciler reconciles a MinecraftServer object
type MinecraftServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *MinecraftServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logutil.FromContextOrNew(ctx).With(
		zap.String("name", req.Name),
		zap.String("namespace", req.Namespace),
		zap.String("controller", "MinecraftServer"))
	ctx = logutil.IntoContext(ctx, log)

	log.Info("beginning reconciliation")

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

	done, err := reconcile.ConfigMap(ctx, r.Client, &server)
	if err != nil {
		return ctrl.Result{}, err
	}
	if done {
		return ctrl.Result{}, nil
	}

	if server.Spec.Dynmap != nil && server.Spec.Dynmap.Enabled {
		done, err := reconcile.DynmapConfigMap(ctx, r.Client, &server)
		if err != nil {
			return ctrl.Result{}, err
		}
		if done {
			return ctrl.Result{}, nil
		}
		done, err = reconcile.DynmapService(ctx, r.Client, &server)
		if err != nil {
			return ctrl.Result{}, err
		}
		if done {
			return ctrl.Result{}, nil
		}
	}

	done, err = reconcile.Service(ctx, r.Client, &server)
	if err != nil {
		return ctrl.Result{}, err
	}
	if done {
		return ctrl.Result{}, nil
	}

	done, err = reconcile.RCONService(ctx, r.Client, &server)
	if err != nil {
		return ctrl.Result{}, err
	}
	if done {
		return ctrl.Result{}, nil
	}

	done, err = reconcile.ReplicaSet(ctx, r.Client, &server)
	if err != nil {
		return ctrl.Result{}, err
	}
	if done {
		return ctrl.Result{}, nil
	}

	// All good, return
	log.Info("All good")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinecraftServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&minecraftv1alpha1.MinecraftServer{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&appsv1.ReplicaSet{}).
		Complete(r)
}
