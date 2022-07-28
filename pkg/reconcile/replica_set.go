package reconcile

import (
	"context"
	"github.com/go-logr/logr"
	"github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReplicaSet(ctx context.Context, k8s client.Client, server *minecraftv1alpha1.MinecraftServer) (bool, error) {
	log, err := logr.FromContext(ctx)
	if err != nil {
		return false, err
	}
	expectedPS := rsForServer(server)

	var actualRS appsv1.ReplicaSet
	err = k8s.Get(ctx, client.ObjectKeyFromObject(&expectedPS), &actualRS)
	if apierrors.IsNotFound(err) {
		log.V(1).Info("ReplicaSet does not exist, creating")
		return true, k8s.Create(ctx, &expectedPS)
	} else if err != nil {
		return false, errors.Wrap(err, "error performing GET on ReplicaSet")
	}

	if !hasCorrectOwnerReference(server, &actualRS) {
		// Set the right owner reference. Adding it to any existing ones.
		actualRS.OwnerReferences = append(actualRS.OwnerReferences, ownerReference(server))
		log.V(1).Info("ReplicaSet owner references incorrect, updating")
		return true, k8s.Update(ctx, &actualRS)
	}

	if !reflect.DeepEqual(expectedPS.Spec, actualRS.Spec) {
		actualRS.Spec = expectedPS.Spec
		log.V(1).Info("ReplicaSet spec incorrect, updating")
		return true, k8s.Update(ctx, &actualRS)
	}

	log.V(2).Info("ReplicaSet OK")
	return false, nil
}

func downloadContainer(url, sha256, filename, volumeMountName string) corev1.Container {
	return corev1.Container{
		Name:  "download",
		Image: "ghcr.io/jameslaverack/download:edge",
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      volumeMountName,
				MountPath: "/download",
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "DOWNLOAD_URL",
				Value: url,
			},
			{
				Name:  "DOWNLOAD_TARGET",
				Value: filepath.Join("/download", filename),
			},
			{
				Name:  "DOWNLOAD_SHA256",
				Value: sha256,
			},
		},
	}
}

func rsForServer(server *v1alpha1.MinecraftServer) appsv1.ReplicaSet {
	const paperJarVolumeName = "paper-jar"

	// TODO Don't hardcode this
	dlContainer := downloadContainer(
		"https://api.papermc.io/v2/projects/paper/versions/1.19/builds/81/downloads/paper-1.19-81.jar",
		"0d39cacc51a77b2b071e1ce862fcbf0b4a4bd668cc7e8b313598d84fa09fabac",
		"paper.jar",
		paperJarVolumeName)

	const configVolumeMountName = "config"
	mainJavaContainer := corev1.Container{
		Name: "minecraft",
		// TODO Configure Java Version
		Image: "eclipse-temurin:17",
		// TODO Make resources configurable
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("6Gi"),
				// No CPU limit to avoid CPU throttling
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("2.5Gi"),
				corev1.ResourceCPU:    resource.MustParse("2"),
			},
		},
		Ports: []corev1.ContainerPort{
			{
				Name:          "minecraft",
				ContainerPort: 25565,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			// This will mount config files, like server.properties, under /etc/minecraft/server.properties
			{
				Name:      configVolumeMountName,
				MountPath: "/etc/minecraft",
			},
			// This will mount the JAR to /usr/local/minecraft/paper.jar
			{
				Name:      paperJarVolumeName,
				MountPath: "/usr/local/minecraft",
			},
		},
	}

	rs := appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            server.Name,
			Namespace:       server.Namespace,
			OwnerReferences: []metav1.OwnerReference{ownerReference(server)},
		},
		Spec: appsv1.ReplicaSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: podLabels(server),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels(server),
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: configVolumeMountName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapNameForServer(server),
									},
								},
							},
						},
						{
							Name: paperJarVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	if server.Spec.World != nil {
		const overworldMountName = "overworld"
		const netherMountName = "nether"
		const theEndMountName = "theEnd"

		rs.Spec.Template.Spec.Volumes = append(rs.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: overworldMountName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: server.Spec.World.Overworld,
				},
			},
			corev1.Volume{
				Name: netherMountName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: server.Spec.World.Nether,
				},
			},
			corev1.Volume{
				Name: theEndMountName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: server.Spec.World.TheEnd,
				},
			})

		mainJavaContainer.VolumeMounts = append(mainJavaContainer.VolumeMounts,
			corev1.VolumeMount{
				Name:      overworldMountName,
				MountPath: "/var/minecraft/world",
			},
			corev1.VolumeMount{
				Name:      netherMountName,
				MountPath: "/var/minecraft/world_nether",
			},
			corev1.VolumeMount{
				Name:      theEndMountName,
				MountPath: "/var/minecraft/world_the_end",
			},
		)
	}

	rs.Spec.Template.Spec.InitContainers = []corev1.Container{dlContainer}
	rs.Spec.Template.Spec.Containers = append(rs.Spec.Template.Spec.Containers, mainJavaContainer)

	return rs
}
