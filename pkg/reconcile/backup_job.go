package reconcile

import (
	"context"
	"strconv"

	"github.com/go-logr/logr"
	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func BackupPod(ctx context.Context, k8s client.Client, backup *minecraftv1alpha1.MinecraftBackup) (bool, error) {
	log, err := logr.FromContext(ctx)
	if err != nil {
		return false, err
	}

	var server minecraftv1alpha1.MinecraftServer
	err = k8s.Get(ctx, client.ObjectKey{Name: backup.Spec.Server.Name, Namespace: backup.Namespace}, &server)
	if client.IgnoreNotFound(err) != nil {
		return false, err
	}
	if apierrors.IsNotFound(err) {
		log.V(1).Info("No server to backup")
		backup.Status.State = minecraftv1alpha1.BackupStateFailed
		return true, k8s.Update(ctx, backup)
	}

	expectedJob := jobForBackup(backup, &server)
	var actualJob batchv1.Job
	err = k8s.Get(ctx, client.ObjectKeyFromObject(expectedJob), &actualJob)
	if client.IgnoreNotFound(err) != nil {
		return false, err
	}
	if apierrors.IsNotFound(err) {
		log.V(1).Info("Job doesn't exist, creating")
		return true, k8s.Create(ctx, expectedJob)
	}

	if actualJob.Status.Failed > 0 {
		if backup.Status.State != minecraftv1alpha1.BackupStateFailed {
			backup.Status.State = minecraftv1alpha1.BackupStateFailed
			return true, k8s.Status().Update(ctx, backup)
		}
		return false, nil
	}

	if actualJob.Status.Succeeded > 0 && backup.Status.State != minecraftv1alpha1.BackupStateComplete {
		backup.Status.State = minecraftv1alpha1.BackupStateComplete
		return true, k8s.Status().Update(ctx, backup)
	}

	return false, nil
}

func jobForBackup(backup *minecraftv1alpha1.MinecraftBackup, server *minecraftv1alpha1.MinecraftServer) *batchv1.Job {
	const overworldMountName = "world-overworld"
	const netherMountName = "world-nether"
	const theEndMountName = "world-the-end"
	const outputMountName = "world-backup"
	rconService := RCONServiceForServer(server)
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:            backup.Name,
			Namespace:       backup.Namespace,
			OwnerReferences: []metav1.OwnerReference{backupOwnerReference(backup)},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					ServiceAccountName: backup.Name,
					Containers: []corev1.Container{
						{
							SecurityContext: securityContext(),
							Name:            "backup-agent",
							Image:           "ghcr.io/jameslaverack/kubernetes-minecraft-operator-backup-agent@sha256:144e42371621b28f98e6327f8b2879a63827ed0b89464cee36813c2c7df9f4c1",
							Env: []corev1.EnvVar{
								{
									Name:  "SERVER_OBJECT_NAME",
									Value: server.Name,
								},
								{
									Name:  "SERVER_OBJECT_NAMESPACE",
									Value: server.Namespace,
								},
								{
									Name:  "BACKUP_NAME",
									Value: backup.Name,
								},
								{
									Name:  "RCON_ADDRESS",
									Value: rconService.Name + ":" + strconv.Itoa(int(rconService.Spec.Ports[0].Port)),
								},
								{
									Name:  "BACKUP_SOURCE_DIR",
									Value: "/var/minecraft/",
								},
								{
									Name:  "BACKUP_DEST_PATH",
									Value: "/var/backups/",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      overworldMountName,
									MountPath: "/var/minecraft/world",
								},
								{
									Name:      netherMountName,
									MountPath: "/var/minecraft/world_nether",
								},
								{
									Name:      theEndMountName,
									MountPath: "/var/minecraft/world_the_end",
								},
								{
									Name:      outputMountName,
									MountPath: "/var/backups/",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: overworldMountName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: server.Spec.World.Overworld,
							},
						},
						{
							Name: netherMountName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: server.Spec.World.Nether,
							},
						},
						{
							Name: theEndMountName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: server.Spec.World.TheEnd,
							},
						},
						{
							Name: outputMountName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: backup.Spec.BackupDestination,
							},
						},
					},
				},
			},
		},
	}
}
