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
	"k8s.io/apimachinery/pkg/util/json"
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

const reconcilerSyncDelay = time.Second * 2

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

func assertEnv(t *testing.T, container corev1.Container, name, value string) {
	require.NotNil(t, container)
	assert.Contains(t, container.Env, corev1.EnvVar{
		Name:  name,
		Value: value,
	})
}

func assertNoEnv(t *testing.T, container corev1.Container, name string) {
	require.NotNil(t, container)
	for _, e := range container.Env {
		if e.Name == name {
			assert.Fail(t, "Found envvar for %s when we didn't expect it", name)
		}
	}
}

func generateTestServer() minecraftv1alpha1.MinecraftServer {
	return minecraftv1alpha1.MinecraftServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: minecraftv1alpha1.MinecraftServerSpec{
			EULA:             minecraftv1alpha1.EULAAcceptanceAccepted,
			MinecraftVersion: "1.18.1",
			Type:             minecraftv1alpha1.ServerTypePaper,
		},
	}
}

func assertOwnerReference(t *testing.T, server *minecraftv1alpha1.MinecraftServer, object metav1.Object) {
	require.Len(t, object.GetOwnerReferences(), 1)
	or := object.GetOwnerReferences()[0]
	assert.Equal(t, server.Name, or.Name)
	assert.Equal(t, "MinecraftServer", or.Kind)
	assert.Equal(t, "minecraft.jameslaverack.com/v1alpha1", or.APIVersion)
	assert.True(t, *or.Controller)
}

func assertConfigMapAttachedToPod(t *testing.T, pod *corev1.Pod, cmName string) {
	// Find the Volume for this config file
	found := false
	volumeName := ""
	for _, v := range pod.Spec.Volumes {
		if v.VolumeSource.ConfigMap != nil &&
			v.VolumeSource.ConfigMap.LocalObjectReference.Name == cmName {
			// Oh hey found it.
			found = true
			volumeName = v.Name
			assert.Empty(t, v.VolumeSource.ConfigMap.Items)
			assert.Nil(t, v.VolumeSource.ConfigMap.Optional)
			break
		}
	}
	assert.True(t, found, "Unable to find Volume on pod for this config map...")
	// Now find the VolumeMount for this volume on the container, there should be exactly one.
	container := pod.Spec.Containers[0]
	assert.Contains(t, container.VolumeMounts, corev1.VolumeMount{
		Name: volumeName,
		// the /config directory is used by the itzg/docker-minecraft-server container to copy data from
		MountPath: "/config",
	})
}

func TestOpsList(t *testing.T) {
	ctx := context.Background()
	k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
	defer teardownFunc()

	server := generateTestServer()
	server.Spec.OpsList = []minecraftv1alpha1.Player{
		{
			// There is a real minecraft user with the name "testplayer", sorry!
			Name: "testplayer",
			UUID: "28a38b40-120c-4883-9122-61a8727ff578",
		},
	}
	err := k8sClient.Create(ctx, &server)
	require.NoError(t, err)

	// TODO Find a better way to know when the reconciler is done
	time.Sleep(reconcilerSyncDelay)

	var configMap corev1.ConfigMap
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &configMap)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &configMap)
	opsFile := configMap.Data["ops.json"]
	assert.NotEmpty(t, opsFile)

	var parsed []struct {
		UUID                string `json:"uuid"`
		Name                string `json:"name"`
		Level               int    `json:"level"`
		BypassesPlayerLimit string `json:"bypassesPlayerLimit"`
	}
	err = json.Unmarshal([]byte(opsFile), &parsed)
	t.Log(parsed)
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	assert.Equal(t, "28a38b40-120c-4883-9122-61a8727ff578", parsed[0].UUID)
	assert.Equal(t, "testplayer", parsed[0].Name)
	assert.Equal(t, 4, parsed[0].Level)
	assert.Equal(t, "false", parsed[0].BypassesPlayerLimit)

	var pod corev1.Pod
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &pod)
	require.NoError(t, err)

	assertConfigMapAttachedToPod(t, &pod, configMap.Name)

	// Check the ENVVARs are set for the config file. As part of startup the itzg/docker-minecraft-server container
	// will move files from /config to /data/config, so we should ensure that we're pointing at those.
	container := pod.Spec.Containers[0]
	assertEnv(t, container, "OPS_FILE", "/data/config/ops.json")
	assertEnv(t, container, "OVERRIDE_OPS", "true")
}

func TestAllowList(t *testing.T) {
	ctx := context.Background()
	k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
	defer teardownFunc()

	server := generateTestServer()
	server.Spec.AllowList = []minecraftv1alpha1.Player{
		{
			// There is a real minecraft user with the name "testplayer", sorry!
			Name: "testplayer",
			UUID: "28a38b40-120c-4883-9122-61a8727ff578",
		},
	}
	err := k8sClient.Create(ctx, &server)
	require.NoError(t, err)

	// TODO Find a better way to know when the reconciler is done
	time.Sleep(reconcilerSyncDelay)

	var configMap corev1.ConfigMap
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &configMap)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &configMap)
	opsFile := configMap.Data["whitelist.json"]
	assert.NotEmpty(t, opsFile)

	var parsed []struct {
		UUID string `json:"uuid"`
		Name string `json:"name"`
	}
	err = json.Unmarshal([]byte(opsFile), &parsed)
	t.Log(parsed)
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	assert.Equal(t, "28a38b40-120c-4883-9122-61a8727ff578", parsed[0].UUID)
	assert.Equal(t, "testplayer", parsed[0].Name)

	var pod corev1.Pod
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &pod)
	require.NoError(t, err)

	assertConfigMapAttachedToPod(t, &pod, configMap.Name)

	// Check the ENVVARs are set for the config file. As part of startup the itzg/docker-minecraft-server container
	// will move files from /config to /data/config, so we should ensure that we're pointing at those.
	container := pod.Spec.Containers[0]
	assertEnv(t, container, "WHITELIST_FILE", "/data/config/whitelist.json")
	assertEnv(t, container, "OVERRIDE_WHITELIST", "true")
	assertEnv(t, container, "ENFORCE_WHITELIST", "TRUE")
}

func TestVanillaTweaksDatapack(t *testing.T) {
	ctx := context.Background()
	k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
	defer teardownFunc()

	server := generateTestServer()
	server.Spec.VanillaTweaks = &minecraftv1alpha1.VanillaTweaks{
		Survival: []string{
			"multiplayer sleep",
		},
	}
	err := k8sClient.Create(ctx, &server)
	require.NoError(t, err)

	// TODO Find a better way to know when the reconciler is done
	time.Sleep(reconcilerSyncDelay)

	var configMap corev1.ConfigMap
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &configMap)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &configMap)
	tweaksFile := configMap.Data["vanilla_tweaks.json"]
	assert.NotEmpty(t, tweaksFile)

	var parsed struct {
		Version string              `json:"version"`
		Packs   map[string][]string `json:"packs"`
	}
	err = json.Unmarshal([]byte(tweaksFile), &parsed)
	t.Log(parsed)
	require.NoError(t, err)
	assert.Equal(t, "1.18", parsed.Version)
	assert.Equal(t, map[string][]string{"survival": []string{"multiplayer sleep"}}, parsed.Packs)

	var pod corev1.Pod
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &pod)
	require.NoError(t, err)

	assertConfigMapAttachedToPod(t, &pod, configMap.Name)

	// Check the ENVVARs are set for the config file. As part of startup the itzg/docker-minecraft-server container
	// will move files from /config to /data/config, so we should ensure that we're pointing at those.
	container := pod.Spec.Containers[0]
	assertEnv(t, container, "VANILLATWEAKS_FILE", "/config/vanilla_tweaks.json")
	assertEnv(t, container, "REMOVE_OLD_VANILLATWEAKS", "true")
}

func TestMountedPVC(t *testing.T) {
	ctx := context.Background()
	k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
	defer teardownFunc()

	pvc := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testpvc",
			Namespace: "default",
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
	err := k8sClient.Create(ctx, &pvc)
	require.NoError(t, err)

	server := generateTestServer()
	server.Spec.World = &minecraftv1alpha1.WorldSpec{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
			ClaimName: pvc.Name,
		},
	}
	err = k8sClient.Create(ctx, &server)
	require.NoError(t, err)

	// TODO Find a better way to know when the reconciler is done
	time.Sleep(reconcilerSyncDelay)

	var pod corev1.Pod
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &pod)
	require.NoError(t, err)

	// Find the volume on the Pod for this PVC
	// Find the Volume for this config file
	found := false
	volumeName := ""
	for _, v := range pod.Spec.Volumes {
		if v.VolumeSource.PersistentVolumeClaim != nil &&
			v.VolumeSource.PersistentVolumeClaim.ClaimName == pvc.Name {
			// Oh hey found it.
			found = true
			volumeName = v.Name
			break
		}
	}
	assert.True(t, found, "Unable to find Pod volume for the world PVC")
	// Find the three expected volume mounts from this volume
	foundWorldVolumeMount := false
	foundNetherVolumeMount := false
	foundEndVolumeMount := false
	container := pod.Spec.Containers[0]
	for _, vm := range container.VolumeMounts {
		if vm.Name == volumeName {
			// Hey it's a mount of our Volume
			switch vm.MountPath {
			case "/data/world":
				foundWorldVolumeMount = true
				assert.Equal(t, "world", vm.SubPath)
			case "/data/world_nether":
				foundNetherVolumeMount = true
				assert.Equal(t, "world_nether", vm.SubPath)
			case "/data/world_the_end":
				foundEndVolumeMount = true
				assert.Equal(t, "world_the_end", vm.SubPath)
			}
			// No default, we tolerate other mounts of the world that aren't these three. We just don't care about them.
		}
	}
	assert.True(t, foundWorldVolumeMount, "Unable to find world volume mount")
	assert.True(t, foundNetherVolumeMount, "Unable to find world_nether volume mount")
	assert.True(t, foundEndVolumeMount, "Unable to find world_the_end volume mount")
}

func TestBasicMinecraftServer(t *testing.T) {
	ctx := context.Background()
	k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
	defer teardownFunc()

	server := generateTestServer()
	err := k8sClient.Create(ctx, &server)
	require.NoError(t, err)

	// TODO Find a better way to know when the reconciler is done
	time.Sleep(reconcilerSyncDelay)

	var pod corev1.Pod
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &pod)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &pod)
	spec := pod.Spec
	require.NotNil(t, spec)

	// Find the container
	require.Len(t, spec.Containers, 1)
	container := spec.Containers[0]

	assert.Equal(t, "itzg/minecraft-server:2022.1.1", container.Image)
	assert.Equal(t, "minecraft", container.Name)

	assertEnv(t, container, "VERSION", server.Spec.MinecraftVersion)
	assertEnv(t, container, "TYPE", string(server.Spec.Type))
	assertEnv(t, container, "MODE", "survival")
	// Important for performance in longer-running containers. We presume the environment will capture logs in the
	// normal way through container STDOUT/STDERR instead.
	assertEnv(t, container, "ENABLE_ROLLING_LOGS", "true")
	// Yes, the allowlist should be enforced even if it's empty. The default behaviour should *not* be to make a public
	// server. That's way too dangerous as a default.
	assertEnv(t, container, "ENFORCE_WHITELIST", "TRUE")

	var service corev1.Service
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &service)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &service)

	assert.Equal(t, corev1.ServiceTypeLoadBalancer, service.Spec.Type)
	require.Len(t, service.Spec.Ports, 1)
	servicePort := service.Spec.Ports[0]
	assert.Equal(t, "minecraft", servicePort.Name)
	assert.Equal(t, int32(25565), servicePort.Port)
	assert.Equal(t, corev1.ProtocolTCP, servicePort.Protocol)

	// Check there is a selector at all
	assert.GreaterOrEqual(t, len(service.Spec.Selector), 1)
	// Check that the Pod will be found by the selector on the service
	for k, v := range service.Spec.Selector {
		assert.Equal(t, v, pod.Labels[k])
	}
}

func TestMOTD(t *testing.T) {
	ctx := context.Background()
	k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
	defer teardownFunc()

	server := generateTestServer()
	motd := "My test message of the day"
	server.Spec.MOTD = motd
	err := k8sClient.Create(ctx, &server)
	require.NoError(t, err)

	// TODO Find a better way to know when the reconciler is done
	time.Sleep(reconcilerSyncDelay)

	var pod corev1.Pod
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &pod)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &pod)
	spec := pod.Spec
	require.NotNil(t, spec)
	container := pod.Spec.Containers[0]

	assertEnv(t, container, "MOTD", motd)
}

func TestViewDistance(t *testing.T) {
	ctx := context.Background()
	k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
	defer teardownFunc()

	server := generateTestServer()
	server.Spec.ViewDistance = 16
	err := k8sClient.Create(ctx, &server)
	require.NoError(t, err)

	// TODO Find a better way to know when the reconciler is done
	time.Sleep(reconcilerSyncDelay)

	var pod corev1.Pod
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &pod)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &pod)
	spec := pod.Spec
	require.NotNil(t, spec)
	container := pod.Spec.Containers[0]

	assertEnv(t, container, "VIEW_DISTANCE", "16")
}

func TestMaxPlayers(t *testing.T) {
	ctx := context.Background()
	k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
	defer teardownFunc()

	server := generateTestServer()
	server.Spec.MaxPlayers = 4
	err := k8sClient.Create(ctx, &server)
	require.NoError(t, err)

	// TODO Find a better way to know when the reconciler is done
	time.Sleep(reconcilerSyncDelay)

	var pod corev1.Pod
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &pod)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &pod)
	spec := pod.Spec
	require.NotNil(t, spec)
	container := pod.Spec.Containers[0]

	assertEnv(t, container, "MAX_PLAYERS", "4")
}

func TestEULA(t *testing.T) {
	testCases := map[string]struct {
		EULA         string
		flagExpected bool
	}{
		"EULA Accepted": {
			EULA:         string(minecraftv1alpha1.EULAAcceptanceAccepted),
			flagExpected: true,
		},
		"EULA Not Accepted": {
			EULA:         string(minecraftv1alpha1.EULAAcceptanceNotAccepted),
			flagExpected: false,
		},
	}

	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
			defer teardownFunc()

			server := generateTestServer()
			server.Spec.EULA = minecraftv1alpha1.EULAAcceptance(tc.EULA)
			err := k8sClient.Create(ctx, &server)
			require.NoError(t, err)

			// TODO Find a better way to know when the reconciler is done
			time.Sleep(reconcilerSyncDelay)

			var pod corev1.Pod
			err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &pod)
			require.NoError(t, err)
			assertOwnerReference(t, &server, &pod)
			spec := pod.Spec
			require.NotNil(t, spec)
			container := pod.Spec.Containers[0]

			if tc.flagExpected {
				assertEnv(t, container, "EULA", "true")
			} else {
				assertNoEnv(t, container, "EULA")
			}
		})
	}
}

func TestReconcilerFixesConfigMap(t *testing.T) {
	ctx := context.Background()
	k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
	defer teardownFunc()

	server := generateTestServer()
	server.Spec.OpsList = []minecraftv1alpha1.Player{
		{
			// There is a real minecraft user with the name "testplayer", sorry!
			Name: "testplayer",
			UUID: "28a38b40-120c-4883-9122-61a8727ff578",
		},
	}
	err := k8sClient.Create(ctx, &server)
	require.NoError(t, err)

	// TODO Find a better way to know when the reconciler is done
	time.Sleep(reconcilerSyncDelay)

	var configMap corev1.ConfigMap
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &configMap)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &configMap)
	opsFile := configMap.Data["ops.json"]

	configMap.Data["ops.json"] = "Bad data"
	err = k8sClient.Update(ctx, &configMap)
	require.NoError(t, err)

	time.Sleep(reconcilerSyncDelay)
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &configMap)
	require.NoError(t, err)
	assert.Equalf(t, opsFile, configMap.Data["ops.json"], "Reconciler did not repair modified config map")
}

func TestReconcilerFixesConfigMapOwnerReference(t *testing.T) {
	ctx := context.Background()
	k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
	defer teardownFunc()

	server := generateTestServer()
	server.Spec.OpsList = []minecraftv1alpha1.Player{
		{
			// There is a real minecraft user with the name "testplayer", sorry!
			Name: "testplayer",
			UUID: "28a38b40-120c-4883-9122-61a8727ff578",
		},
	}
	err := k8sClient.Create(ctx, &server)
	require.NoError(t, err)

	// TODO Find a better way to know when the reconciler is done
	time.Sleep(reconcilerSyncDelay)

	var configMap corev1.ConfigMap
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &configMap)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &configMap)
	configMap.OwnerReferences = []metav1.OwnerReference{}
	err = k8sClient.Update(ctx, &configMap)
	require.NoError(t, err)

	time.Sleep(reconcilerSyncDelay)
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &configMap)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &configMap)
}

func TestReconcilerFixesPodOwnerReference(t *testing.T) {
	ctx := context.Background()
	k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
	defer teardownFunc()

	server := generateTestServer()
	err := k8sClient.Create(ctx, &server)
	require.NoError(t, err)

	// TODO Find a better way to know when the reconciler is done
	time.Sleep(reconcilerSyncDelay)

	var pod corev1.Pod
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &pod)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &pod)
	pod.OwnerReferences = []metav1.OwnerReference{}
	err = k8sClient.Update(ctx, &pod)
	require.NoError(t, err)

	time.Sleep(reconcilerSyncDelay)
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &pod)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &pod)
}

func TestReconcilerFixesPodLabels(t *testing.T) {
	ctx := context.Background()
	k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
	defer teardownFunc()

	server := generateTestServer()
	err := k8sClient.Create(ctx, &server)
	require.NoError(t, err)

	// TODO Find a better way to know when the reconciler is done
	time.Sleep(reconcilerSyncDelay)

	var pod corev1.Pod
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &pod)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &pod)

	// Add a new label *and* modify an old one
	pod.Labels["app"] = "foo"
	pod.Labels["my-custom-label"] = "bar"

	err = k8sClient.Update(ctx, &pod)
	require.NoError(t, err)

	time.Sleep(reconcilerSyncDelay)
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &pod)
	require.NoError(t, err)
	// We expect that app will have been corrected, but the custom label to have remained
	assert.Equal(t, "minecraft", pod.Labels["app"])
	assert.Equal(t, "bar", pod.Labels["my-custom-label"])
}

func TestReconcilerFixesServiceOwnerReference(t *testing.T) {
	ctx := context.Background()
	k8sClient, teardownFunc := setupTestingEnvironment(ctx, t)
	defer teardownFunc()

	server := generateTestServer()
	err := k8sClient.Create(ctx, &server)
	require.NoError(t, err)

	// TODO Find a better way to know when the reconciler is done
	time.Sleep(reconcilerSyncDelay)

	var service corev1.Service
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &service)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &service)
	service.OwnerReferences = []metav1.OwnerReference{}
	err = k8sClient.Update(ctx, &service)
	require.NoError(t, err)

	time.Sleep(reconcilerSyncDelay)
	err = k8sClient.Get(ctx, client.ObjectKeyFromObject(&server), &service)
	require.NoError(t, err)
	assertOwnerReference(t, &server, &service)
}
