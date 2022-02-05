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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sync"
	"testing"
	"time"
	//+kubebuilder:scaffold:imports
)

func setupTestingEnvironment(ctx context.Context, t *testing.T) (client.Client, func()) {
	l := zap.New(zap.UseDevMode(true))
	logf.SetLogger(l)

	// Setup testing environment
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	err = minecraftv1alpha1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	//+kubebuilder:scaffold:scheme

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)
	require.NotNil(t, k8sClient)

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)

	minecraftServerReconciler := MinecraftServerReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}
	err = minecraftServerReconciler.SetupWithManager(mgr)

	managerContext, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := mgr.Start(managerContext)
		require.NoError(t, err)
	}()

	return k8sClient, func() {
		// calling cancel here will cancel the managerContext, which will cause the blocking call to mgr.Start in the
		// goroutine above to exit, which will cause that goroutine to exit, which will execute the deferred wg.Done(),
		// which will then unblock the wg.Wait().
		cancel()
		wg.Wait()

		err := testEnv.Stop()
		if err != nil {
			panic(err)
		}
	}
}

func TestMinecraftServer(t *testing.T) {
	ctx := context.Background()
	k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
	defer teardownFunc()

	const ns = "default"
	// Create a Minecraft Server resource
	pvcObj := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testpvc",
			Namespace: ns,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}
	err := k8sClient.Create(ctx, &pvcObj)
	require.NoError(t, err)

	serverObj := minecraftv1alpha1.MinecraftServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: ns,
		},
		Spec: minecraftv1alpha1.MinecraftServerSpec{
			EULA:             minecraftv1alpha1.EULAAcceptanceAccepted,
			MinecraftVersion: "1.18.1",
			Type:             minecraftv1alpha1.ServerTypePaper,
			World: &minecraftv1alpha1.WorldSpec{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcObj.Name,
				},
			},
			OpsList: []minecraftv1alpha1.Player{
				{
					// There is a real minecraft user with the name "testplayer", sorry!
					Name: "testplayer",
					UUID: "28a38b40-120c-4883-9122-61a8727ff578",
				},
			},
		},
	}
	err = k8sClient.Create(ctx, &serverObj)
	require.NoError(t, err)

	// TODO Find a better way to know when the reconciler is done
	time.Sleep(time.Second * 10)

	assertOwnerReference := func(object metav1.Object) {
		require.Len(t, object.GetOwnerReferences(), 1)
		or := object.GetOwnerReferences()[0]
		assert.Equal(t, serverObj.Name, or.Name)
		assert.Equal(t, "MinecraftServer", or.Kind)
		assert.Equal(t, "minecraft.jameslaverack.com/v1alpha1", or.APIVersion)
		assert.True(t, *or.Controller)
	}

	// We expect the server to create a few things
	var configMap corev1.ConfigMap
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&serverObj), &configMap)
	require.NoError(t, err)
	assertOwnerReference(&configMap)
	assert.NotEmpty(t, configMap.Data["ops.json"])

	var pod corev1.Pod
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&serverObj), &pod)
	require.NoError(t, err)
	assertOwnerReference(&pod)
	spec := pod.Spec
	require.NotNil(t, spec)

	// Find the container
	require.Len(t, spec.Containers, 1)
	container := spec.Containers[0]

	assert.Equal(t, "itzg/minecraft-server:2022.1.1", container.Image)
	assert.Equal(t, "minecraft", container.Name)

	assertEnv := func(name, value string) {
		assert.Contains(t, container.Env, corev1.EnvVar{
			Name:  name,
			Value: value,
		})
	}

	assertEnv("VERSION", serverObj.Spec.MinecraftVersion)
	assertEnv("TYPE", string(serverObj.Spec.Type))
	assertEnv("MODE", "survival")
	assertEnv("ENABLE_ROLLING_LOGS", "true")

	var service corev1.Service
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&serverObj), &service)
	require.NoError(t, err)
	assertOwnerReference(&service)

	assert.Equal(t, corev1.ServiceTypeLoadBalancer, service.Spec.Type)
	require.Len(t, service.Spec.Ports, 1)
	servicePort := service.Spec.Ports[0]
	assert.Equal(t, "minecraft", servicePort.Name)
	assert.Equal(t, int32(25565), servicePort.Port)
	assert.Equal(t, corev1.ProtocolTCP, servicePort.Protocol)

}
