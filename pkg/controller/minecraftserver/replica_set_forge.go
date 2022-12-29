package minecraftserver

import (
	"context"
	"net/url"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
)

func forgeDownloadUrl(server *v1alpha1.MinecraftServer) (*url.URL, error) {
	// https://maven.minecraftforge.net/net/minecraftforge/forge/1.18.2-40.1.80/forge-1.18.2-40.1.80-installer.jar
	versionIdntifier := server.Spec.MinecraftVersion + "-" + server.Spec.Forge.ForgeVersion
	path, err := url.JoinPath("net",
		"minecraftforge",
		"forge",
		url.PathEscape(versionIdntifier),
		url.PathEscape("forge-"+versionIdntifier+"-installer.jar"))
	if err != nil {
		return nil, err
	}
	return &url.URL{
		Scheme: "https",
		Host:   "maven.minecraftforge.net",
		Path:   path,
	}, nil
}

func rsForServerTypeForge(ctx context.Context, server *v1alpha1.MinecraftServer) (appsv1.ReplicaSet, error) {
	const forgeInstallerVolumeName = "forge-installer-jar"
	const installerTmp = "installer-tmp"
	const modpackZipVolumeName = "modpack-zip-volume"
	const forgeWorkingDirVolumeName = "forge-workingdir"
	const configVolumeMountName = "config"
	const worldMountName = "world-overworld"
	const dataPacksMountName = "data-packs"

	url, err := forgeDownloadUrl(server)
	if err != nil {
		return appsv1.ReplicaSet{}, err
	}

	forgeDownloadContainer := downloadContainer(url.String(), server.Spec.Forge.ForgeInstallerSHA256Sum, "forge-installer.jar", forgeInstallerVolumeName)
	copyConfigContainer := copyConfigContainer(configVolumeMountName, forgeWorkingDirVolumeName)
	forgeInstallerContainer := corev1.Container{
		Name: "forge-installer",
		// TODO Configure Java Version
		Image: "eclipse-temurin:17",
		Args: []string{
			"java",
			///////////////////////////////
			// Flags here are flags to Java
			///////////////////////////////
			"-jar",
			"/usr/local/minecraft/forge-installer.jar",
			////////////////////////////////////////////////////////////
			// Flags after this point are flags to the Forge installer, and not Java
			////////////////////////////////////////////////////////////
			// Disable the GUI, no need in a container
			"--installServer=/run/minecraft"},
		WorkingDir: "/usr/local/minecraft",
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
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      forgeInstallerVolumeName,
				MountPath: "/usr/local/minecraft",
			},
			{
				Name:      forgeWorkingDirVolumeName,
				MountPath: "/run/minecraft",
			},
			{
				Name:      installerTmp,
				MountPath: "/tmp",
			},
		},
	}
	modpackDownloadContainer := downloadContainer(
		server.Spec.Forge.ModpackZipURL,
		server.Spec.Forge.ModpackZipSHA256Sum,
		"modpack.zip",
		modpackZipVolumeName)
	modpackUnzipContainer := corev1.Container{
		Name: "modpack-unzip",
		// TODO Configure Java Version
		Image: "busybox",
		Args: []string{
			"unzip", "/usr/local/modpack/modpack.zip", "-d/run/minecraft"},
		// TODO Make resources configurable
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("1Gi"),
				// No CPU limit to avoid CPU throttling
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("1Gi"),
				corev1.ResourceCPU:    resource.MustParse("2"),
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      modpackZipVolumeName,
				MountPath: "/usr/local/modpack",
			},
			{
				Name:      forgeWorkingDirVolumeName,
				MountPath: "/run/minecraft",
			},
		},
	}

	initContainers := []corev1.Container{
		forgeDownloadContainer,
		forgeInstallerContainer,
		modpackDownloadContainer,
		modpackUnzipContainer,
		copyConfigContainer}

	mainJavaContainer := corev1.Container{
		Name: "minecraft",
		// TODO Configure Java Version
		Image: "eclipse-temurin:17",
		Args: []string{
			"sh",
			"run.sh",
			// Set the world directory to be /var/minecraft
			//"--world-container=/var/minecraft",
			// Set the plugin directory to be /usr/local/minecraft/plugins
			//"--plugins=/usr/local/minecraft/plugins",
			// Disable the on-disk logging, we'll use STDOUT logging always
			//"--log-append=false",
			// Disable the GUI, no need in a container
			"--nogui"},
		// Paper expects to be able to write all kinds of stuff to it's working directory, so we give it a dedicated
		// scratch dir for it's use under /run/minecraft.
		WorkingDir: "/run/minecraft",
		// TODO Make resources configurable
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("12Gi"),
				// No CPU limit to avoid CPU throttling
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("8Gi"),
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
			// This gives forge a writeable runtime directory, this is used as the working directory.
			{
				Name:      forgeWorkingDirVolumeName,
				MountPath: "/run/minecraft",
			},
			// Mount the various world directories under /var/minecraft
			{
				Name:      worldMountName,
				MountPath: "/run/minecraft/world",
			},
			{
				Name:      dataPacksMountName,
				MountPath: "/run/minecraft/world/datapacks",
			},
		},
	}

	var replicas int32 = 1
	rs := appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            server.Name,
			Namespace:       server.Namespace,
			OwnerReferences: []metav1.OwnerReference{serverOwnerReference(server)},
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
							Name: forgeInstallerVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: forgeWorkingDirVolumeName,
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
							Name: modpackZipVolumeName,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: installerTmp,
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
				Name: worldMountName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: server.Spec.World.Overworld,
				},
			})
	} else {
		// No world to persist, so mount EmptyDir volumes.
		rs.Spec.Template.Spec.Volumes = append(rs.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: worldMountName,
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

	rs.Spec.Template.Spec.InitContainers = initContainers
	rs.Spec.Template.Spec.Containers = append(rs.Spec.Template.Spec.Containers, mainJavaContainer)

	// Put the security context on *everything*
	for i := range rs.Spec.Template.Spec.InitContainers {
		rs.Spec.Template.Spec.InitContainers[i].SecurityContext = SecurityContext()
	}
	for i := range rs.Spec.Template.Spec.Containers {
		rs.Spec.Template.Spec.Containers[i].SecurityContext = SecurityContext()
	}

	return rs, nil
}
