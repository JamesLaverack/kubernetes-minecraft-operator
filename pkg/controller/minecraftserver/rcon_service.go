package minecraftserver

import (
	"context"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/logutil"
)

func RCONService(ctx context.Context, k8s client.Client, server *minecraftv1alpha1.MinecraftServer) (bool, error) {
	log := logutil.FromContextOrNew(ctx)

	expectedService := RCONServiceForServer(server)
	log = log.With(zap.String("rcon-service-name", expectedService.Name))

	var actualService corev1.Service
	err := k8s.Get(ctx, client.ObjectKeyFromObject(&expectedService), &actualService)
	if client.IgnoreNotFound(err) != nil {
		return false, err
	}

	if apierrors.IsNotFound(err) {
		log.Info("Service doesn't exist, creating")
		return true, k8s.Create(ctx, &expectedService)
	}

	// Check service for integrity
	if !hasCorrectOwnerReference(server, &actualService) {
		log.Info("Service owner references incorrect, adjusting")
		actualService.OwnerReferences = append(actualService.OwnerReferences, serverOwnerReference(server))
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

	log.Debug("RCON Service OK")
	return false, nil
}

func RCONServiceForServer(server *minecraftv1alpha1.MinecraftServer) corev1.Service {
	prefer := corev1.IPFamilyPolicyPreferDualStack
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            server.Name + "-rcon",
			Namespace:       server.Namespace,
			OwnerReferences: []metav1.OwnerReference{serverOwnerReference(server)},
		},
		Spec: corev1.ServiceSpec{
			IPFamilyPolicy: &prefer,
			Selector:       podLabels(server),
			Ports: []corev1.ServicePort{
				{
					Name:     "rcon",
					Port:     25575,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}

	return service
}
