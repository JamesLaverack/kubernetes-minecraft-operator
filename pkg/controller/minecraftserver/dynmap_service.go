package minecraftserver

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/logutil"
)

func DynmapService(ctx context.Context, k8s client.Client, server *minecraftv1alpha1.MinecraftServer) (bool, error) {
	log := logutil.FromContextOrNew(ctx)

	expectedService := dynmapServiceForServer(server)

	var actualService corev1.Service
	err := k8s.Get(ctx, client.ObjectKeyFromObject(&expectedService), &actualService)
	if client.IgnoreNotFound(err) != nil {
		return false, err
	}

	if apierrors.IsNotFound(err) {
		log.Info("Dynmap service doesn't exist, creating")
		return true, k8s.Create(ctx, &expectedService)
	}

	// Check service for integrity
	if !hasCorrectOwnerReference(server, &actualService) {
		log.Info("Dynmap service owner references incorrect, updating")
		actualService.OwnerReferences = append(actualService.OwnerReferences, serverOwnerReference(server))
		return true, k8s.Update(ctx, &actualService)
	}

	for _, expectedPort := range expectedService.Spec.Ports {
		foundPort := false
		for i, actualPort := range actualService.Spec.Ports {
			if expectedPort.Name == actualPort.Name {
				foundPort = true
				if expectedPort.Protocol != actualPort.Protocol {
					log.Info("Dynmap service port protocol incorrect, updating")
					actualService.Spec.Ports[i].Protocol = expectedPort.Protocol
					return true, k8s.Update(ctx, &actualService)
				}
				if expectedPort.Port != actualPort.Port {
					log.Info("Dynmap service port number incorrect, updating")
					actualService.Spec.Ports[i].Port = expectedPort.Port
					return true, k8s.Update(ctx, &actualService)
				}
				if expectedPort.NodePort != 0 && expectedPort.NodePort != actualPort.NodePort {
					log.Info("Dynmap service node port number incorrect, updating")
					actualService.Spec.Ports[i].NodePort = expectedPort.NodePort
					return true, k8s.Update(ctx, &actualService)
				}
				break
			}
		}
		if !foundPort {
			log.Info("Dynamp service port missing, adding")
			actualService.Spec.Ports = append(actualService.Spec.Ports, expectedPort)
			return true, k8s.Update(ctx, &actualService)
		}
	}

	if actualService.Spec.Type != expectedService.Spec.Type {
		log.Info("Dynmap service type incorrect, updating")
		actualService.Spec.Type = expectedService.Spec.Type
		return true, k8s.Update(ctx, &actualService)
	}

	log.Debug("Service OK")
	return false, nil
}

func dynmapServiceForServer(server *minecraftv1alpha1.MinecraftServer) corev1.Service {
	prefer := corev1.IPFamilyPolicyPreferDualStack
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            server.Name + "-dynmap",
			Namespace:       server.Namespace,
			OwnerReferences: []metav1.OwnerReference{serverOwnerReference(server)},
		},
		Spec: corev1.ServiceSpec{
			IPFamilyPolicy: &prefer,
			Type:           corev1.ServiceTypeClusterIP,
			Selector:       podLabels(server),
			Ports: []corev1.ServicePort{
				{
					Name:       "dynmap",
					Port:       80,
					TargetPort: intstr.FromInt(8123),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	return service
}
