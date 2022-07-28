package reconcile

import (
	"context"
	"fmt"
	"github.com/blang/semver"
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	minecraftv1alpha1 "github.com/jameslaverack/kubernetes-minecraft-operator/api/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

func configMapNameForServer(server *minecraftv1alpha1.MinecraftServer) string {
	return server.Name
}

func ConfigMap(ctx context.Context, k8s client.Client, server *minecraftv1alpha1.MinecraftServer) (bool, error) {
	log, err := logr.FromContext(ctx)
	if err != nil {
		return false, err
	}
	data, err := configMapData(server.Spec)
	if err != nil {
		return false, err
	}

	expectedName := types.NamespacedName{
		Name: configMapNameForServer(server),
		Namespace: server.Namespace,
	}

	var actualConfigMap corev1.ConfigMap
	err = k8s.Get(ctx, expectedName, &actualConfigMap)
	if apierrors.IsNotFound(err) {
		log.V(1).Info("ConfigMap does not exist, creating")
		expectedConfigMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            expectedName.Name,
				Namespace:       expectedName.Namespace,
				OwnerReferences: []metav1.OwnerReference{ownerReference(server)},
			},
			Data: data,
		}
		return true, k8s.Create(ctx, &expectedConfigMap)
	} else if err != nil {
		return false, errors.Wrap(err, "error performing GET on ConfigMap")
	}

	if !hasCorrectOwnerReference(server, &actualConfigMap) {
		log.V(1).Info("ConfigMap owner references incorrect, updating")
		actualConfigMap.OwnerReferences = append(actualConfigMap.OwnerReferences, ownerReference(server))
		return true, k8s.Update(ctx, &actualConfigMap)
	}

	if !reflect.DeepEqual(actualConfigMap.Data, data) {
		log.V(1).Info("ConfigMap data incorrect, updating")
		actualConfigMap.Data = data
		return true, k8s.Update(ctx, &actualConfigMap)
	}

	log.V(2).Info("ConfigMap OK")
	return false, nil
}

func configMapData(spec minecraftv1alpha1.MinecraftServerSpec) (map[string]string, error) {
	config := make(map[string]string)

	config["server.properties"] = fmt.Sprintf(`
motd=%s
`,
	spec.MOTD)

	if spec.EULA == minecraftv1alpha1.EULAAcceptanceAccepted {
		config["eula.txt"] = "true"
	}

	if len(spec.AllowList) > 0 {
		// We can directly marshall the Player objects
		d, err := json.Marshal(spec.AllowList)
		if err != nil {
			return nil, err
		}
		config["whitelist.json"] = string(d)
	}

	if len(spec.OpsList) > 0 {
		type op struct {
			UUID                string `json:"uuid,omitempty"`
			Name                string `json:"name,omitempty"`
			Level               int    `json:"level"`
			BypassesPlayerLimit string `json:"bypassesPlayerLimit"`
		}
		ops := make([]op, len(spec.OpsList))
		for i, o := range spec.OpsList {
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

	if spec.VanillaTweaks != nil {
		var minorVersion string
		// This is janky
		if strings.Count(spec.MinecraftVersion, ".") >= 2 {
			version, err := semver.Parse(spec.MinecraftVersion)
			if err != nil {
				return nil, errors.Wrap(err, "Unable to parse semver version")
			}
			minorVersion = strconv.Itoa(int(version.Major)) + "." + strconv.Itoa(int(version.Minor))
		} else {
			minorVersion = spec.MinecraftVersion
		}
		d, err := json.Marshal(struct {
			Version string                          `json:"version"`
			Packs   minecraftv1alpha1.VanillaTweaks `json:"packs"`
		}{
			Version: minorVersion,
			Packs:   *spec.VanillaTweaks,
		})
		if err != nil {
			return nil, err
		}
		config["vanilla_tweaks.json"] = string(d)
	}

	if spec.Monitoring != nil && spec.Monitoring.Type == minecraftv1alpha1.MonitoringTypePrometheusServiceMonitor {
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
