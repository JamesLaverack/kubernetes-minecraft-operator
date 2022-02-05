package reconcile

import (
	"context"
	"github.com/jameslaverack/minecraft-operator/api/v1alpha1"
	minecraftv1alpha1 "github.com/jameslaverack/minecraft-operator/api/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

func ReconcilePod(ctx context.Context, logger *zap.SugaredLogger, reader client.Reader, server *minecraftv1alpha1.MinecraftServer, cm *corev1.ConfigMap) (*corev1.Pod, ReconcileAction, error) {
	expectedPod := podForServer(server, cm)

	var actualPod corev1.Pod
	err := reader.Get(ctx, client.ObjectKeyFromObject(&expectedPod), &actualPod)
	if apierrors.IsNotFound(err) {
		// Pretty simple, just create it
		return &expectedPod,
			func(ctx context.Context, logger *zap.SugaredLogger, writer client.Writer) (ctrl.Result, error) {
				logger.Info("Creating Minecraft Pod")
				return ctrl.Result{}, writer.Create(ctx, &expectedPod)
			},
			nil
	}
	if err != nil {
		return nil, nil, err
	}

	// The pod exists, check integrity. A Pod is a bit special in Kubernetes in that it can't really be updated in-place
	// so for most changes we'll just delete it and allow the controller to recreate it on a future pass.

	if !hasCorrectOwnerReference(server, &actualPod) {
		// Set the right owner reference. Adding it to any existing ones.
		actualPod.OwnerReferences = append(actualPod.OwnerReferences, ownerReference(server))
		return &actualPod,
			func(ctx context.Context, logger *zap.SugaredLogger, writer client.Writer) (ctrl.Result, error) {
				logger.Info("Setting owner reference on pod")
				return ctrl.Result{}, writer.Update(ctx, &actualPod)
			},
			nil
	}

	if !hasCorrectLabels(&expectedPod, &actualPod) {
		// patch the labels
		for k, v := range expectedPod.Labels {
			actualPod.Labels[k] = v
		}
		return &actualPod,
			func(ctx context.Context, logger *zap.SugaredLogger, writer client.Writer) (ctrl.Result, error) {
				logger.Info("Setting labels correctly on pod")
				return ctrl.Result{}, writer.Update(ctx, &actualPod)
			},
			nil
	}

	// TODO Detect and fix Pod changes
	//if !reflect.DeepEqual(expectedPod.Spec, actualPod.Spec) {
	//	return &actualPod,
	//		func(ctx context.Context, logger *zap.SugaredLogger, writer client.Writer) (ctrl.Result, error) {
	//			logger.Info("Pod spec is incorrect, deleting")
	//			return ctrl.Result{}, writer.Delete(ctx, &actualPod)
	//		},
	//		nil
	//}

	logger.Debug("Pod all okay")
	return &actualPod, nil, nil
}

func podForServer(server *v1alpha1.MinecraftServer, configMap *corev1.ConfigMap) corev1.Pod {
	const configVolumeMountName = "config"
	container := corev1.Container{
		Name: "minecraft",
		// TODO Configure version somehow
		Image: "itzg/minecraft-server:2022.1.1",
		Env: []corev1.EnvVar{
			{
				Name:  "INIT_MEMORY",
				Value: "2g",
			},
			{
				Name:  "MAX_MEMORY",
				Value: "6g",
			},
			{
				Name:  "VERSION",
				Value: server.Spec.MinecraftVersion,
			},
			{
				Name:  "TYPE",
				Value: string(server.Spec.Type),
			},
			{
				// In theory this is redundant as the /data directory isn't mounted or persisted directly, so files
				// like /data/server.properties will be destroyed on restart. But better safe than sorry with config
				// file changes.
				Name:  "OVERRIDE_SERVER_PROPERTIES",
				Value: "true",
			},
			{
				// TODO Make configurable if you want a public server I guess
				Name:  "ENABLE_WHITELIST",
				Value: "TRUE",
			},
			{
				Name:  "WHITELIST_FILE",
				Value: "/data/config/whitelist.json",
			},
			{
				Name:  "OVERRIDE_WHITELIST",
				Value: "true",
			},
			{
				Name:  "OPS_FILE",
				Value: "/data/config/ops.json",
			},
			{
				Name:  "OVERRIDE_OPS",
				Value: "true",
			},
			{
				// TODO Make configurable if you want a public server I guess
				Name:  "ENFORCE_WHITELIST",
				Value: "TRUE",
			},
			{
				Name:  "ENABLE_RCON",
				Value: "false",
			},
			{
				Name:  "FORCE_GAMEMODE",
				Value: "true",
			},
			{
				Name: "SPAWN_PROTECTION",
				// This disables spawn protection
				Value: "0",
			},
			{
				// TODO Make configurable
				Name:  "MODE",
				Value: "survival",
			},
			{
				// TODO Find a way to turn off filesystem logging entirely. In a Kubernetes cluster we can just collect
				//      the logs from STDOUT.
				Name:  "ENABLE_ROLLING_LOGS",
				Value: "true",
			},
			{
				Name:  "SYNC_SKIP_NEWER_IN_DESTINATION",
				Value: "true",
			},
		},
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
			{
				Name:      configVolumeMountName,
				MountPath: "/config",
			},
		},
	}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
			Labels:    podLabels(server),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{},
			Volumes: []corev1.Volume{
				{
					Name: configVolumeMountName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configMap.Name,
							},
						},
					},
				},
			},
		},
	}

	if server.Spec.World != nil {
		const levelName = "world"
		const levelNameNether = "world_nether"
		const levelNameTheEnd = "world_the_end"
		const worldVolumeMountName = "world"
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			corev1.Volume{
				Name: worldVolumeMountName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: server.Spec.World.PersistentVolumeClaim,
				},
			})
		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{
				Name: worldVolumeMountName,
				// TODO Account for the fact that we might be running on Windows and therefore filepath.Join will do the
				//      wrong kind of filepath joining?
				MountPath: filepath.Join("/data/", levelName),
				SubPath:   levelName,
			},
			corev1.VolumeMount{
				Name:      worldVolumeMountName,
				MountPath: filepath.Join("/data/", levelNameNether),
				SubPath:   levelNameNether,
			},
			corev1.VolumeMount{
				Name:      worldVolumeMountName,
				MountPath: filepath.Join("/data/", levelNameTheEnd),
				SubPath:   levelNameTheEnd,
			},
		)
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "LEVEL",
			Value: levelName,
		})
	}

	if server.Spec.MOTD != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "MOTD",
			Value: server.Spec.MOTD,
		})
	}

	if server.Spec.EULA == v1alpha1.EULAAcceptanceAccepted {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "EULA",
			Value: "true",
		})
	}

	if server.Spec.MaxPlayers > 0 {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "MAX_PLAYERS",
			Value: strconv.Itoa(server.Spec.MaxPlayers),
		})
	}

	if server.Spec.ViewDistance > 0 {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "VIEW_DISTANCE",
			Value: strconv.Itoa(server.Spec.ViewDistance),
		})
	}

	if server.Spec.VanillaTweaks != nil {
		container.Env = append(container.Env,
			corev1.EnvVar{
				Name:  "VANILLATWEAKS_FILE",
				Value: "/config/vanilla_tweaks.json",
			},
			corev1.EnvVar{
				Name:  "REMOVE_OLD_VANILLATWEAKS",
				Value: "true",
			})
	}

	spigetResources := make([]string, 0)

	if server.Spec.Monitoring != nil && server.Spec.Monitoring.Enabled {
		// This magic number is the Spigot plugin ID for the Prometheus Exporter plugin
		// https://www.spigotmc.org/resources/prometheus-exporter.36618
		spigetResources = append(spigetResources, "36618")
		container.Ports = append(container.Ports,
			corev1.ContainerPort{
				Name:          "metrics",
				ContainerPort: 9225,
			})
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			corev1.Volume{
				Name: "prometheusExporterConfig",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMap.Name,
						},
						Items: []corev1.KeyToPath{
							{
								Key:  "prometheus_exporter_config.yaml",
								Path: "config.yml",
							},
						},
					},
				},
			})
		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{
				Name:      "prometheusExporterConfig",
				MountPath: "/data/plugins/PrometheusExporter",
				SubPath:   "config.yml",
			})
	}

	// Do this last, just before return
	if len(spigetResources) > 0 {
		container.Env = append(container.Env, corev1.EnvVar{
			// Yes this is 'SPIGET' not 'SPIGOT'.
			Name:  "SPIGET_RESOURCES",
			Value: strings.Join(spigetResources, ","),
		})
	}
	pod.Spec.Containers = append(pod.Spec.Containers, container)

	return pod
}
