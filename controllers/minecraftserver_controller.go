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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MinecraftServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
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
			Name:      server.Name,
			Namespace: server.Namespace,
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

	return ctrl.Result{}, nil
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
		Complete(r)
}
