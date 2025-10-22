package config

import (
	context "context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ControllerConfig struct {
	// Add your config fields here
	SomeOption string
}

// DefaultConfig returns the default configuration
func DefaultConfig() *ControllerConfig {
	return &ControllerConfig{
		SomeOption: "default-value",
	}
}

// LoadConfigFromConfigMap loads configuration from a ConfigMap in the given namespace and name
// If the ConfigMap or keys are missing, it falls back to defaults
func LoadConfigFromConfigMap(ctx context.Context, c client.Client, namespace, name string) (*ControllerConfig, error) {
	cfg := DefaultConfig()
	cm := &corev1.ConfigMap{}
	err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, cm)
	if err != nil {
		return cfg, fmt.Errorf("could not get configmap: %w (using defaults)", err)
	}
	if val, ok := cm.Data["SomeOption"]; ok {
		cfg.SomeOption = val
	}
	return cfg, nil
}
