package minecraftbackup

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/logutil"
)

func BackupRBAC(ctx context.Context, k8s client.Client, backup *minecraftv1alpha1.MinecraftBackup) (bool, error) {
	log := logutil.FromContextOrNew(ctx)

	// Service Account
	expectedSA := serviceAccountForBackup(backup)
	var actualSA corev1.ServiceAccount
	err := k8s.Get(ctx, client.ObjectKeyFromObject(expectedSA), &actualSA)
	if client.IgnoreNotFound(err) != nil {
		return false, err
	}
	if apierrors.IsNotFound(err) {
		log.Info("SA doesn't exist, creating")
		return true, k8s.Create(ctx, expectedSA)
	}

	// Role
	expectedRole := roleForBackup(backup)
	var actualRole rbacv1.Role
	err = k8s.Get(ctx, client.ObjectKeyFromObject(expectedRole), &actualRole)
	if client.IgnoreNotFound(err) != nil {
		return false, err
	}
	if apierrors.IsNotFound(err) {
		log.Info("Role doesn't exist, creating")
		return true, k8s.Create(ctx, expectedRole)
	}

	// RoleBinding
	expectedRoleBinding := roleBindingForBackup(backup)
	var actualRoleBinding rbacv1.RoleBinding
	err = k8s.Get(ctx, client.ObjectKeyFromObject(expectedRoleBinding), &actualRoleBinding)
	if client.IgnoreNotFound(err) != nil {
		return false, err
	}
	if apierrors.IsNotFound(err) {
		log.Info("Role doesn't exist, creating")
		return true, k8s.Create(ctx, expectedRoleBinding)
	}

	return false, nil
}

func serviceAccountForBackup(backup *minecraftv1alpha1.MinecraftBackup) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            backup.Name,
			Namespace:       backup.Namespace,
			OwnerReferences: []metav1.OwnerReference{backupOwnerReference(backup)},
		},
	}
}

func roleForBackup(backup *minecraftv1alpha1.MinecraftBackup) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            backup.Name,
			Namespace:       backup.Namespace,
			OwnerReferences: []metav1.OwnerReference{backupOwnerReference(backup)},
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"get", "list", "update"},
				APIGroups:     []string{minecraftv1alpha1.GroupVersion.Group},
				Resources:     []string{"minecraftservers"},
				ResourceNames: []string{backup.Spec.Server.Name},
			},
		},
	}
}

func roleBindingForBackup(backup *minecraftv1alpha1.MinecraftBackup) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            backup.Name,
			Namespace:       backup.Namespace,
			OwnerReferences: []metav1.OwnerReference{backupOwnerReference(backup)},
		},
		RoleRef: rbacv1.RoleRef{
			Name:     backup.Name,
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				APIGroup:  corev1.GroupName,
				Name:      backup.Name,
				Namespace: backup.Namespace,
			},
		},
	}
}
