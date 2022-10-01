package controller

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
)

func setupTestingEnvironment(ctx context.Context, t *testing.T) (client.WithWatch, func()) {
	l := zap.New(zap.UseDevMode(true))
	logf.SetLogger(l)

	// Setup testing environment
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "crd")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	err = minecraftv1alpha1.AddToScheme(scheme.Scheme)
	require.NoError(t, err)

	k8s, err := client.NewWithWatch(cfg, client.Options{Scheme: scheme.Scheme})
	require.NoError(t, err)
	require.NotNil(t, k8s)

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

	return k8s, func() {
		// calling cancel here will cancel the managerContext, which will cause the blocking call to mgr.Start in the
		// goroutine above to exit, which will cause that goroutine to exit, which will execute the deferred wg.Done(),
		// which will then unblock the wg.Wait().
		cancel()
		wg.Wait()
		_ = testEnv.Stop()
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
			MinecraftVersion: "1.19",
			Type:             minecraftv1alpha1.ServerTypePaper,
			GameMode:         minecraftv1alpha1.GameModeSurvival,
			AccessMode:       minecraftv1alpha1.AccessModePublic,
			Service: &minecraftv1alpha1.ServiceSpec{
				Type: minecraftv1alpha1.ServiceTypeClusterIP,
			},
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

const timeout = time.Second * 10
const tick = time.Second

func TestBasicServer(t *testing.T) {
	ctx := context.Background()
	k8s, teardown := setupTestingEnvironment(ctx, t)
	defer teardown()

	server := generateTestServer()
	err := k8s.Create(ctx, &server)
	require.NoError(t, err)

	// ConfigMap
	var configMap corev1.ConfigMap
	require.Eventually(t, func() bool {
		return k8s.Get(ctx, client.ObjectKeyFromObject(&server), &configMap) == nil
	}, timeout, tick)

	assertOwnerReference(t, &server, &configMap)
	opsFile := configMap.Data["server.properties"]
	assert.NotEmpty(t, opsFile)

	// ReplicaSet
	var replicaSet appsv1.ReplicaSet
	require.Eventually(t, func() bool {
		return k8s.Get(ctx, client.ObjectKeyFromObject(&server), &replicaSet) == nil
	}, timeout, tick)

	assertOwnerReference(t, &server, &replicaSet)
	assert.Equal(t, "eclipse-temurin:17", replicaSet.Spec.Template.Spec.Containers[0].Image)

	// Service
	var service corev1.Service
	require.Eventually(t, func() bool {
		return k8s.Get(ctx, client.ObjectKeyFromObject(&server), &service) == nil
	}, timeout, tick)

	assertOwnerReference(t, &server, &service)
}
