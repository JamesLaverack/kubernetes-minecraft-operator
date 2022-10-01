package reconcile

import (
	"context"
	"path/filepath"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/bibliothek"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/logutil"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/vanillatweaks"
)

func ReplicaSet(ctx context.Context, k8s client.Client, server *minecraftv1alpha1.MinecraftServer) (bool, error) {
	log := logutil.FromContextOrNew(ctx)
	expectedPS, err := rsForServer(ctx, server)
	if err != nil {
		return false, err
	}

	var actualRS appsv1.ReplicaSet
	err = k8s.Get(ctx, client.ObjectKeyFromObject(&expectedPS), &actualRS)
	if apierrors.IsNotFound(err) {
		log.Info("ReplicaSet does not exist, creating")
		return true, k8s.Create(ctx, &expectedPS)
	} else if err != nil {
		return false, errors.Wrap(err, "error performing GET on ReplicaSet")
	}

	if !hasCorrectOwnerReference(server, &actualRS) {
		// Set the right owner reference. Adding it to any existing ones.
		actualRS.OwnerReferences = append(actualRS.OwnerReferences, ownerReference(server))
		log.Info("ReplicaSet owner references incorrect, updating")
		return true, k8s.Update(ctx, &actualRS)
	}

	log.Debug("ReplicaSet OK")
	return false, nil
}

// Spigot really, *really* wants to be able to write to its config files. So we copy them over from the configmap to
// the Spigot server's working directory in /run/minecraft to let it run all over them.
// TODO Handle live changes to these files, maybe with some kind of sidecar?
func copyConfigContainer(configVolumeMountName, paperWorkingDirVolumeName string) corev1.Container {
	return corev1.Container{
		Name:  "copy-config",
		Image: "busybox",
		// We use sh here to get file globbing with the *.
		Args: []string{"sh", "-c", "cp /etc/minecraft/* /run/minecraft/"},
		VolumeMounts: []corev1.VolumeMount{
			// This will mount config files, like server.properties, under /etc/minecraft/server.properties
			{
				Name:      configVolumeMountName,
				MountPath: "/etc/minecraft",
			},
			// This gives paper a writeable runtime directory, this is used as the working directory.
			{
				Name:      paperWorkingDirVolumeName,
				MountPath: "/run/minecraft",
			},
		},
	}
}

func copyDynmapConfigContainer(dynmapConfigVolumeMountName, dynmapDataVolumeName string) corev1.Container {
	return corev1.Container{
		Name:  "copy-dynmap-config",
		Image: "busybox",
		Args:  []string{"sh", "-c", "cp /etc/dynmap/* /usr/local/minecraft/plugins/dynmap"},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      dynmapConfigVolumeMountName,
				MountPath: "/etc/dynmap",
			},
			{
				Name:      dynmapDataVolumeName,
				MountPath: "/usr/local/minecraft/plugins/dynmap",
			},
		},
	}
}

func vanillaTweaksDatapackContainer(ctx context.Context, datapacksVolumeMountName string, version string, tweaks *minecraftv1alpha1.VanillaTweaks) (corev1.Container, error) {
	url, err := vanillatweaks.GetDatapackDownloadURL(ctx, version, tweaks.Datapacks)
	if err != nil {
		return corev1.Container{}, err
	}

	return corev1.Container{
		Name:  "install-vanillatweaks",
		Image: "busybox",
		Args:  []string{"sh", "-c", "cd /var/minecraft/world/datapacks && wget -O vt.zip '" + url + "' && unzip vt.zip && rm vt.zip"},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      datapacksVolumeMountName,
				MountPath: "/var/minecraft/world/datapacks",
			},
		},
	}, nil
}

func spigetInstallContainer(pluginResourceID, pluginName, pluginsVolumeMountName string) corev1.Container {
	return corev1.Container{
		Name:  "install-dynmap",
		Image: "busybox",
		Args:  []string{"sh", "-c", "cd /usr/local/minecraft/plugins && wget -O " + pluginName + ".jar 'https://api.spiget.org/v2/resources/" + pluginResourceID + "/download'"},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      pluginsVolumeMountName,
				MountPath: "/usr/local/minecraft/plugins",
			},
		},
	}
}

func downloadContainer(url, sha256, filename, volumeMountName string) corev1.Container {
	return corev1.Container{
		Name:            "download",
		Image:           "ghcr.io/jameslaverack/download:edge",
		ImagePullPolicy: corev1.PullAlways,
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

func securityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		Privileged:               pointer.Bool(false),
		RunAsUser:                pointer.Int64(1000),
		RunAsGroup:               pointer.Int64(1000),
		RunAsNonRoot:             pointer.Bool(true),
		ReadOnlyRootFilesystem:   pointer.Bool(true),
		AllowPrivilegeEscalation: pointer.Bool(false),
	}
}

func rsForServer(ctx context.Context, server *v1alpha1.MinecraftServer) (appsv1.ReplicaSet, error) {
	const paperJarVolumeName = "paper-jar"
	const paperWorkingDirVolumeName = "paper-workingdir"
	const configVolumeMountName = "config"
	const overworldMountName = "world-overworld"
	const netherMountName = "world-nether"
	const theEndMountName = "world-the-end"
	const dataPacksMountName = "data-packs"
	const pluginsMountName = "plugins"

	latestBuild, err := bibliothek.LatestBuildForVersion(server.Spec.MinecraftVersion)
	if err != nil {
		return appsv1.ReplicaSet{}, err
	}
	url, sha256, err := bibliothek.GetDownloadURLAndSHA256(server.Spec.MinecraftVersion, latestBuild)
	if err != nil {
		return appsv1.ReplicaSet{}, err
	}

	paperDownloadContainer := downloadContainer(url, sha256, "paper.jar", paperJarVolumeName)
	copyConfigContainer := copyConfigContainer(configVolumeMountName, paperWorkingDirVolumeName)

	initContainers := []corev1.Container{paperDownloadContainer, copyConfigContainer}

	mainJavaContainer := corev1.Container{
		Name: "minecraft",
		// TODO Configure Java Version
		Image: "eclipse-temurin:17",
		Args: []string{
			"java",
			///////////////////////////////
			// Flags here are flags to Java
			///////////////////////////////
			"-jar",
			"/usr/local/minecraft/paper.jar",
			////////////////////////////////////////////////////////////
			// Flags after this point are flags to PaperMC, and not Java
			////////////////////////////////////////////////////////////
			// Set the world directory to be /var/minecraft
			"--world-container=/var/minecraft",
			// Set the plugin directory to be /usr/local/minecraft/plugins
			"--plugins=/usr/local/minecraft/plugins",
			// Disable the on-disk logging, we'll use STDOUT logging always
			"--log-append=false",
			// Disable the GUI, no need in a container
			"--nogui"},
		// Paper expects to be able to write all kinds of stuff to it's working directory, so we give it a dedicated
		// scratch dir for it's use under /run/minecraft.
		WorkingDir: "/run/minecraft",
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
				Protocol:      corev1.ProtocolTCP,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			// This will mount config files, like server.properties, under /etc/minecraft/server.properties
			{
				Name:      configVolumeMountName,
				MountPath: "/etc/minecraft",
			},
			// This gives paper a writeable runtime directory, this is used as the working directory.
			{
				Name:      paperWorkingDirVolumeName,
				MountPath: "/run/minecraft",
			},
			// This will mount the JAR to /usr/local/minecraft/paper.jar
			{
				Name:      paperJarVolumeName,
				MountPath: "/usr/local/minecraft",
			},
			{
				Name:      pluginsMountName,
				MountPath: "/usr/local/minecraft/plugins",
			},
			// Mount the various world directories under /var/minecraft
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
			// Mount the datapacks at /var/minecraft/world/datapacks
			{
				Name:      dataPacksMountName,
				MountPath: "/var/minecraft/world/datapacks",
			},
		},
	}

	var replicas int32 = 1
	rs := appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            server.Name,
			Namespace:       server.Namespace,
			OwnerReferences: []metav1.OwnerReference{ownerReference(server)},
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &replicas,
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
						{
							Name: paperWorkingDirVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: dataPacksMountName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: pluginsMountName,
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
		// Persistent world, so mount so PVCs
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
	} else {
		// No world to persist, so mount EmptyDir volumes.
		rs.Spec.Template.Spec.Volumes = append(rs.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: overworldMountName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			corev1.Volume{
				Name: netherMountName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			corev1.Volume{
				Name: theEndMountName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			})
	}

	if server.Spec.VanillaTweaks != nil {
		vtDownloadContainer, err := vanillaTweaksDatapackContainer(ctx, dataPacksMountName, server.Spec.MinecraftVersion, server.Spec.VanillaTweaks)
		if err != nil {
			return appsv1.ReplicaSet{}, err
		}
		initContainers = append(initContainers, vtDownloadContainer)
	}

	if server.Spec.Dynmap != nil && server.Spec.Dynmap.Enabled {
		initContainers = append(initContainers, spigetInstallContainer("274", "dynmap", pluginsMountName))
		mainJavaContainer.Ports = append(mainJavaContainer.Ports,
			corev1.ContainerPort{
				Name:          "dynmap",
				ContainerPort: 8123,
				Protocol:      corev1.ProtocolTCP,
			})
		const dynmapConfigMountName = "dynmap-config"
		rs.Spec.Template.Spec.Volumes = append(rs.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: dynmapConfigMountName,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: dynmapConfigMapNameForServer(server),
						},
					},
				},
			})
		// Use an init container to move the mounted dynmap config into the runtime plugins dir
		const dynmapDataMountName = "dynmap-data"
		initContainers = append(initContainers, copyDynmapConfigContainer(dynmapConfigMountName, dynmapDataMountName))

		mainJavaContainer.VolumeMounts = append(
			mainJavaContainer.VolumeMounts,
			corev1.VolumeMount{
				Name:      dynmapDataMountName,
				MountPath: "/usr/local/minecraft/plugins/dynmap/",
			})

		if server.Spec.Dynmap.MapStorage != nil {
			rs.Spec.Template.Spec.Volumes = append(rs.Spec.Template.Spec.Volumes,
				corev1.Volume{
					Name: dynmapDataMountName,
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: server.Spec.Dynmap.MapStorage,
					},
				})
		} else {
			rs.Spec.Template.Spec.Volumes = append(rs.Spec.Template.Spec.Volumes,
				corev1.Volume{
					Name: dynmapDataMountName,
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				})
		}
	}

	rs.Spec.Template.Spec.InitContainers = initContainers
	rs.Spec.Template.Spec.Containers = append(rs.Spec.Template.Spec.Containers, mainJavaContainer)

	// Put the security context on *everything*
	for i := range rs.Spec.Template.Spec.InitContainers {
		rs.Spec.Template.Spec.InitContainers[i].SecurityContext = securityContext()
	}
	for i := range rs.Spec.Template.Spec.Containers {
		rs.Spec.Template.Spec.Containers[i].SecurityContext = securityContext()
	}

	return rs, nil
}
