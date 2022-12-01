package minecraftserver

import (
	"context"
	"strconv"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"

	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/logutil"
	"github.com/jameslaverack/kubernetes-minecraft-operator/pkg/propertiesfile"
)

func configMapNameForServer(server *minecraftv1alpha1.MinecraftServer) string {
	return server.Name
}

func ConfigMap(ctx context.Context, k8s client.Client, server *minecraftv1alpha1.MinecraftServer) (bool, error) {
	log := logutil.FromContextOrNew(ctx)

	data, err := configMapData(*server)
	if err != nil {
		return false, err
	}

	expectedName := types.NamespacedName{
		Name:      configMapNameForServer(server),
		Namespace: server.Namespace,
	}

	var actualConfigMap corev1.ConfigMap
	err = k8s.Get(ctx, expectedName, &actualConfigMap)
	if apierrors.IsNotFound(err) {
		log.Info("ConfigMap does not exist, creating")
		expectedConfigMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            expectedName.Name,
				Namespace:       expectedName.Namespace,
				OwnerReferences: []metav1.OwnerReference{serverOwnerReference(server)},
			},
			Data: data,
		}
		return true, k8s.Create(ctx, &expectedConfigMap)
	} else if err != nil {
		return false, errors.Wrap(err, "error performing GET on ConfigMap")
	}

	if !hasCorrectOwnerReference(server, &actualConfigMap) {
		log.Info("ConfigMap owner references incorrect, updating")
		actualConfigMap.OwnerReferences = append(actualConfigMap.OwnerReferences, serverOwnerReference(server))
		return true, k8s.Update(ctx, &actualConfigMap)
	}

	//if !reflect.DeepEqual(actualConfigMap.Data, data) {
	//	log.Info("ConfigMap data incorrect, updating")
	//	actualConfigMap.Data = data
	//	return true, k8s.Update(ctx, &actualConfigMap)
	//}

	log.Debug("ConfigMap OK")
	return false, nil
}

func configMapData(server minecraftv1alpha1.MinecraftServer) (map[string]string, error) {
	config := make(map[string]string)

	props := make(map[string]string, 0)
	props["enable-rcon"] = "true"
	// TODO Use a real passsword
	props["rcon.password"] = "password"
	if server.Spec.MOTD != "" {
		props["motd"] = server.Spec.MOTD
	}
	if server.Spec.GameMode != "" {
		props["gamemode"] = strings.ToLower(string(server.Spec.GameMode))
	}
	if server.Spec.MaxPlayers > 0 {
		props["max-players"] = strconv.Itoa(server.Spec.MaxPlayers)
	}
	if server.Spec.AccessMode == minecraftv1alpha1.AccessModeAllowListOnly {
		props["enforce-whitelist"] = "true"
		props["white-list"] = "true"
	}
	if server.Spec.World != nil && server.Spec.World.Seed != "" {
		props["level-seed"] = server.Spec.World.Seed
	}
	config["server.properties"] = propertiesfile.Write(props)

	// We always write a eula.txt file, but we *only* put "true" in it if the MinecraftServer object has had the EULA
	// explicitly accepted.
	if server.Spec.EULA == minecraftv1alpha1.EULAAcceptanceAccepted {
		config["eula.txt"] = "eula=true"
	} else {
		config["eula.txt"] = "eula=false"
	}

	if len(server.Spec.AllowList) > 0 {
		// We can directly marshall the Player objects
		d, err := json.Marshal(server.Spec.AllowList)
		if err != nil {
			return nil, err
		}
		config["whitelist.json"] = string(d)
	}

	if len(server.Spec.OpsList) > 0 {
		type op struct {
			UUID                string `json:"uuid,omitempty"`
			Name                string `json:"name,omitempty"`
			Level               int    `json:"level"`
			BypassesPlayerLimit string `json:"bypassesPlayerLimit"`
		}
		ops := make([]op, len(server.Spec.OpsList))
		for i, o := range server.Spec.OpsList {
			ops[i] = op{
				UUID:                o.UUID,
				Name:                o.Name,
				Level:               4,
				BypassesPlayerLimit: "false",
			}
		}
		d, err := json.Marshal(ops)
		if err != nil {
			return nil, err
		}
		config["ops.json"] = string(d)
	}

	if server.Spec.Monitoring != nil && server.Spec.Monitoring.Type == minecraftv1alpha1.MonitoringTypePrometheusServiceMonitor {
		// prometheus-exporter plugin file
		c := map[string]interface{}{
			// This is the important bit, by default this plugin binds to localhost which isn't useful in K8s
			"host": "0.0.0.0",
			"port": 9225,
			"enable_metrics": map[string]interface{}{
				// These are the default settings (as of the time of writing)
				"jvm_threads":           true,
				"jvm_gc":                true,
				"players_total":         true,
				"entities_total":        true,
				"living_entities_total": true,
				"loaded_chunks_total":   true,
				"jvm_memory":            true,
				"players_online_total":  true,
				"tps":                   true,
				"tick_duration_average": true,
				"tick_duration_median":  true,
				"tick_duration_min":     false,
				"tick_duration_max":     true,
				"player_online":         false,
				"player_statistic":      false,
			},
		}
		d, err := yaml.Marshal(c)
		if err != nil {
			return nil, err
		}
		config["prometheus_exporter_config.yaml"] = string(d)
	}

	// We need this for comparison later, because K8s will store an empty map as a nil (on the configmap data field
	// anyway).
	if len(config) == 0 {
		return nil, nil
	}
	return config, nil
}
