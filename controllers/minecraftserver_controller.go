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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"path/filepath"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

// MinecraftServerReconciler reconciles a MinecraftServer object
type MinecraftServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=minecraft.jameslaverack.com,resources=minecraftservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=minecraft.jameslaverack.com,resources=minecraftservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=minecraft.jameslaverack.com,resources=minecraftservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *MinecraftServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var server minecraftv1alpha1.MinecraftServer
	if err := r.Get(ctx, req.NamespacedName, &server); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	configFiles, err := configMapForServer(server.Spec)
	if err != nil {
		return ctrl.Result{}, err
	}

	desiredConfigMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            server.Name,
			Namespace:       server.Namespace,
			OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(&server, minecraftv1alpha1.GroupVersion.WithKind("MinecraftServer"))},
		},
		Data: configFiles,
	}
	var actualConfigMap corev1.ConfigMap
	if err := r.Get(ctx, client.ObjectKeyFromObject(&desiredConfigMap), &actualConfigMap); err != nil {
		if apierrors.IsNotFound(err) {
			// No CM, make it and exit.
			return ctrl.Result{}, r.Create(ctx, &desiredConfigMap)
		}
		// Some error on the GET that *isn't* a not found. Take no further action.
		return ctrl.Result{}, err
	}
	// Compare the contents of the actual CM to ours.
	// TODO handle keys in the files in different orders, JSON encoding differences, etc.
	if !reflect.DeepEqual(desiredConfigMap.Data, actualConfigMap.Data) ||
		!reflect.DeepEqual(desiredConfigMap.OwnerReferences, actualConfigMap.OwnerReferences) {
		// ConfigMap data isn't correct. Update it.
		return ctrl.Result{}, r.Update(ctx, &desiredConfigMap)
	}

	desiredPod := podForServer(server.Name, server.Namespace, server.Spec)
	desiredPod.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&server, minecraftv1alpha1.GroupVersion.WithKind("MinecraftServer"))}
	var actualPod corev1.Pod
	if err := r.Get(ctx, client.ObjectKeyFromObject(&desiredPod), &actualPod); err != nil {
		if apierrors.IsNotFound(err) {
			// No Pod, make it and exit
			return ctrl.Result{}, r.Create(ctx, &desiredPod)
		}
		return ctrl.Result{}, err
	}
	// Compare the contents of the actual Pod to ours
	// TODO Don't use DeepEqual because a webhook could mutate some field and we might end up in some infinite loop
	// TODO Check the labels too
	if !reflect.DeepEqual(desiredPod.Spec, actualPod.Spec) ||
		!reflect.DeepEqual(desiredPod.OwnerReferences, actualPod.OwnerReferences) {
		return ctrl.Result{}, r.Update(ctx, &desiredPod)
	}

	desiredService := serviceForServer(server.Name, server.Namespace, server.Spec)
	desiredService.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*metav1.NewControllerRef(&server, minecraftv1alpha1.GroupVersion.WithKind("MinecraftServer"))}
	var actualService corev1.Service
	if err := r.Get(ctx, client.ObjectKeyFromObject(&desiredService), &actualService); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.Create(ctx, &desiredService)
		}
		return ctrl.Result{}, err
	}
	if !reflect.DeepEqual(desiredService.Spec, actualService.Spec) ||
		!reflect.DeepEqual(desiredService.OwnerReferences, actualService.OwnerReferences) {
		return ctrl.Result{}, r.Update(ctx, &desiredService)
	}

	// All good, return
	return ctrl.Result{}, nil
}

func serviceForServer(name, namespace string, spec minecraftv1alpha1.MinecraftServerSpec) corev1.Service {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app":       "minecraft",
				"minecraft": name,
			},
		},
		Spec: corev1.ServiceSpec{
			// TODO Make configurable
			Type: corev1.ServiceTypeLoadBalancer,
			Selector: map[string]string{
				// TODO Pull these labels into a supporting function or something
				"app":       "minecraft",
				"minecraft": name,
			},
			Ports: []corev1.ServicePort{
				{
					Name: "minecraft",
					Port: 25565,
				},
			},
		},
	}

	if spec.ExternalServiceIP != "" {
		service.Spec.ExternalIPs = []string{spec.ExternalServiceIP}
	}

	return service
}

func podForServer(name, namespace string, spec minecraftv1alpha1.MinecraftServerSpec) corev1.Pod {
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
				Name:  "TYPE",
				Value: string(spec.Type),
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
				Name:  "ENFORCE_WHITELIST",
				Value: "true",
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
				Name:  "ENABLE_ROLLING_LOGS",
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
			// TODO Add metrics
			{
				Name:          "minecraft",
				ContainerPort: 25565,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			// TODO Don't mount configmap volume, or make the configmap, if there's nothing in it (e.g., when there's
			//      no whitelist).
			{
				Name:      configVolumeMountName,
				MountPath: "/config",
			},
		},
	}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app":       "minecraft",
				"minecraft": name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{},
			Volumes: []corev1.Volume{
				{
					Name: configVolumeMountName,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: name,
							},
						},
					},
				},
			},
		},
	}

	if spec.World != nil {
		const levelName = "world"
		const worldVolumeMountName = "world"
		pod.Spec.Volumes = append(pod.Spec.Volumes,
			corev1.Volume{
				Name: worldVolumeMountName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: spec.World.PersistentVolumeClaim,
				},
			})
		container.VolumeMounts = append(container.VolumeMounts,
			corev1.VolumeMount{
				Name: worldVolumeMountName,
				// TODO Account for the fact that we might be running on Windows and therefore filepath.Join will do the
				//      wrong kind of filepath joining?
				MountPath: filepath.Join("/data/", levelName),
			})
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "LEVEL",
			Value: levelName,
		})
	}

	if spec.MOTD != "" {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "MOTD",
			Value: spec.MOTD,
		})
	}

	if spec.EULA == minecraftv1alpha1.EULAAcceptanceAccepted {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "EULA",
			Value: "true",
		})
	}

	if spec.MaxPlayers > 0 {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "MAX_PLAYERS",
			Value: strconv.Itoa(spec.MaxPlayers),
		})
	}

	if spec.ViewDistance > 0 {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "VIEW_DISTANCE",
			Value: strconv.Itoa(spec.ViewDistance),
		})
	}

	// Do this last, just before return
	pod.Spec.Containers = append(pod.Spec.Containers, container)

	return pod
}

func configMapForServer(spec minecraftv1alpha1.MinecraftServerSpec) (map[string]string, error) {
	serverProperties := make(map[string]string)
	if spec.MOTD != "" {
		serverProperties["motd"] = spec.MOTD
	}
	// Needed for the file system to sync up
	serverProperties["level-name"] = "world"
	// TODO configure on CRD
	serverProperties["gamemode"] = "survival"
	// TODO configure on CRD
	serverProperties["difficulty"] = "normal"
	if len(spec.AllowList) > 0 {
		// Minecraft uses the term "whitelist", but we use "allowlist" wherever possible
		serverProperties["white-list"] = "true"
	} else {
		serverProperties["white-list"] = "false"
	}
	if spec.MaxPlayers > 0 {
		serverProperties["max-players"] = strconv.Itoa(spec.MaxPlayers)
	}
	if spec.ViewDistance > 0 {
		serverProperties["view-distance"] = strconv.Itoa(spec.ViewDistance)
	}
	// TODO Maybe use RCONS for something useful
	serverProperties["enable-rcon"] = "false"

	config := make(map[string]string)
	serverPropertiesString := ""
	for k, v := range serverProperties {
		serverPropertiesString = serverPropertiesString + k + "=" + v + "\n"
	}
	config["server.properties"] = serverPropertiesString

	if len(spec.AllowList) > 0 {
		// We can directly marshall the Player objects
		d, err := json.Marshal(spec.AllowList)
		if err != nil {
			return nil, err
		}
		config["whitelist.json"] = string(d)
	}

	return config, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MinecraftServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&minecraftv1alpha1.MinecraftServer{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
