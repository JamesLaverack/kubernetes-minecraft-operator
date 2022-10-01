package controller

import (
	"context"

	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/logutil"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/reconcile"
)

// MinecraftServerReconciler reconciles a MinecraftServer object
type MinecraftBackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *MinecraftBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logutil.FromContextOrNew(ctx).With(
		zap.String("name", req.Name),
		zap.String("namespace", req.Namespace),
		zap.String("controller", "MinecraftServer"))
	ctx = logutil.IntoContext(ctx, log)

	log.Info("beginning reconciliation")

	var backup minecraftv1alpha1.MinecraftBackup
	if err := r.Get(ctx, req.NamespacedName, &backup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if backup.Status.State == "" {
		backup.Status.State = minecraftv1alpha1.BackupStatePending
		return ctrl.Result{}, r.Client.Status().Update(ctx, &backup)
	}

	// We'll now create each resource we need. In general we'll "reconcile" each resource in turn. If there's work to be
	// done we'll do it an exit instantly. This is because this function is triggered on changes to owned resources, so
	// the act of creating or modifying an owned resource will cause this function to be called again anyway.

	done, err := reconcile.BackupRBAC(ctx, r.Client, &backup)
	if err != nil {
		return ctrl.Result{}, err
	}
	if done {
		return ctrl.Result{}, nil
	}

	done, err = reconcile.BackupPod(ctx, r.Client, &backup)
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
func (r *MinecraftBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&minecraftv1alpha1.MinecraftBackup{}).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}
