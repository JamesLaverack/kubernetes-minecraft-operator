package minecraftserver

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/logutil"
)

func Service(ctx context.Context, k8s client.Client, server *minecraftv1alpha1.MinecraftServer) (bool, error) {
	log := logutil.FromContextOrNew(ctx)

	var actualService corev1.Service
	err := k8s.Get(ctx, client.ObjectKeyFromObject(server), &actualService)
	if client.IgnoreNotFound(err) != nil {
		return false, err
	}

	if server.Spec.Service == nil || server.Spec.Service.Type == minecraftv1alpha1.ServiceTypeNone {
		// We should make sure we *don't* have a service.
		if apierrors.IsNotFound(err) {
			log.Debug("Service OK")
			return false, nil
		}
		log.Info("Service exists when it shouldn't, removing")
		return true, k8s.Delete(ctx, &actualService)
	}

	expectedService := serviceForServer(server)

	if apierrors.IsNotFound(err) {
		log.Info("Service doesn't exist, creating")
		return true, k8s.Create(ctx, &expectedService)
	}

	// Check service for integrity
	if !hasCorrectOwnerReference(server, &actualService) {
		log.Info("Service owner references incorrect, updating")
		actualService.OwnerReferences = append(actualService.OwnerReferences, serverOwnerReference(server))
		return true, k8s.Update(ctx, &actualService)
	}

	if actualService.Spec.Type != expectedService.Spec.Type {
		log.Info("Service type incorrect, updating")
		actualService.Spec.Type = expectedService.Spec.Type
		return true, k8s.Update(ctx, &actualService)
	}

	for _, expectedPort := range expectedService.Spec.Ports {
		foundPort := false
		for i, actualPort := range actualService.Spec.Ports {
			if expectedPort.Name == actualPort.Name {
				foundPort = true
				if expectedPort.Protocol != actualPort.Protocol {
					log.Info("Service port protocol incorrect, updating")
					actualService.Spec.Ports[i].Protocol = expectedPort.Protocol
					return true, k8s.Update(ctx, &actualService)
				}
				if expectedPort.Port != actualPort.Port {
					log.Info("Service port number incorrect, updating")
					actualService.Spec.Ports[i].Port = expectedPort.Port
					return true, k8s.Update(ctx, &actualService)
				}
				if expectedPort.NodePort != 0 && expectedPort.NodePort != actualPort.NodePort {
					log.Info("Service node port number incorrect, updating")
					actualService.Spec.Ports[i].NodePort = expectedPort.NodePort
					return true, k8s.Update(ctx, &actualService)
				}
				break
			}
		}
		if !foundPort {
			log.Info("Service port missing, adding")
			actualService.Spec.Ports = append(actualService.Spec.Ports, expectedPort)
			return true, k8s.Update(ctx, &actualService)
		}
	}

	log.Debug("Service OK")
	return false, nil
}

func serviceForServer(server *minecraftv1alpha1.MinecraftServer) corev1.Service {
	prefer := corev1.IPFamilyPolicyPreferDualStack
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            server.Name,
			Namespace:       server.Namespace,
			OwnerReferences: []metav1.OwnerReference{serverOwnerReference(server)},
		},
		Spec: corev1.ServiceSpec{
			IPFamilyPolicy: &prefer,
			Type:           corev1.ServiceType(server.Spec.Service.Type),
			Selector:       podLabels(server),
			Ports: []corev1.ServicePort{
				{
					Name:     "minecraft",
					Port:     25565,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}

	if server.Spec.Service.MinecraftNodePort != nil && *server.Spec.Service.MinecraftNodePort > 0 {
		service.Spec.Ports[0].NodePort = *server.Spec.Service.MinecraftNodePort
	}

	return service
}
