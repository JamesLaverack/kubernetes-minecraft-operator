package reconcile

import (
	"context"
	"github.com/blang/semver"
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	minecraftv1alpha1 "github.com/jameslaverack/minecraft-operator/api/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

func ReconcileConfigMap(ctx context.Context, logger logr.Logger, reader client.Reader, server *minecraftv1alpha1.MinecraftServer) (*corev1.ConfigMap, ReconcileAction, error) {
	data, err := configMapData(server.Spec)
	if err != nil {
		return nil, nil, err
	}

	expectedName := client.ObjectKeyFromObject(server)

	var actualConfigMap corev1.ConfigMap
	err = reader.Get(ctx, expectedName, &actualConfigMap)
	if apierrors.IsNotFound(err) {
		// Simple, just make the map!
		expectedConfigMap := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:            expectedName.Name,
				Namespace:       expectedName.Namespace,
				OwnerReferences: []metav1.OwnerReference{ownerReference(server)},
			},
			Data: data,
		}
		return &expectedConfigMap,
			func(ctx context.Context, logger logr.Logger, writer client.Writer) (ctrl.Result, error) {
				logger.Info("Creating config map with Minecraft configuration files")
				return ctrl.Result{}, writer.Create(ctx, &expectedConfigMap)
			},
			nil

	}
	// Check for some other error, for example a permissions problem
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to get configmap from API server")
	}

	// ConfigMap already exists, so verify its integrity
	if !hasCorrectOwnerReference(server, &actualConfigMap) {
		// Set the right owner reference. Adding it to any existing ones.
		actualConfigMap.OwnerReferences = append(actualConfigMap.OwnerReferences, ownerReference(server))
		return &actualConfigMap,
			func(ctx context.Context, logger logr.Logger, writer client.Writer) (ctrl.Result, error) {
				logger.Info("Setting owner reference on configmap")
				return ctrl.Result{}, writer.Update(ctx, &actualConfigMap)
			},
			nil
	}
	if !reflect.DeepEqual(actualConfigMap.Data, data) {
		// Correct the data field
		actualConfigMap.Data = data
		return &actualConfigMap,
			func(ctx context.Context, logger logr.Logger, writer client.Writer) (ctrl.Result, error) {
				logger.Info("Correcting data on configmap")
				return ctrl.Result{}, writer.Update(ctx, &actualConfigMap)
			},
			nil
	}
	// We don't set or need any labels or annotations, so don't bother checking those

	logger.V(1).Info("Configmap all okay")
	return &actualConfigMap, nil, nil
}

func configMapData(spec minecraftv1alpha1.MinecraftServerSpec) (map[string]string, error) {
	config := make(map[string]string)

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
		version, err := semver.Parse(spec.MinecraftVersion)
		if err != nil {
			return nil, errors.Wrap(err, "Unable to parse semver version")
		}
		minorVersion := strconv.Itoa(int(version.Major)) + "." + strconv.Itoa(int(version.Minor))
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

	if spec.Monitoring != nil && spec.Monitoring.Enabled {
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

	return config, nil
}
