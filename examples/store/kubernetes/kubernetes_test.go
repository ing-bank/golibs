package main

import (
	"context"
	"fmt"

	"github.com/ing-bank/golibs/pkg/store/backends/kubernetes"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Example() {
	ctx := context.Background()

	// Configure the Kubernetes backend for ConfigMaps
	cfg := kubernetes.Config{
		Namespace: "default",
		Group:     "",
		Version:   "v1",
		Resource:  "configmaps",
	}

	// Create a fake store for this example (doesn't require a real cluster)
	// For production use: store, err := kubernetes.NewForConfig(cfg)
	store := kubernetes.NewFake[*v1.ConfigMap](cfg)

	// Create a ConfigMap with actual v1.ConfigMap type
	configMap := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-config",
			Namespace: "default",
			Labels: map[string]string{
				"app": "example",
			},
		},
		Data: map[string]string{
			"database.host": "postgres.example.com",
			"database.port": "5432",
			"app.name":      "my-application",
		},
	}

	// Create the ConfigMap
	if err := store.Create(ctx, "app-config", configMap); err != nil {
		fmt.Println("Create error:", err)
		return
	}
	fmt.Println("Created ConfigMap")

	// Read the ConfigMap back
	retrieved, err := store.Read(ctx, "app-config")
	if err != nil {
		fmt.Println("Read error:", err)
		return
	}
	fmt.Println("Retrieved ConfigMap:", retrieved.Name)
	fmt.Println("Data entries:", len(retrieved.Data))

	// Update the ConfigMap
	retrieved.Data["log.level"] = "debug"
	if err := store.Update(ctx, "app-config", retrieved); err != nil {
		fmt.Println("Update error:", err)
		return
	}
	fmt.Println("Updated ConfigMap")

	// List all ConfigMaps
	items, err := store.List(ctx)
	if err != nil {
		fmt.Println("List error:", err)
		return
	}
	fmt.Println("ConfigMaps in namespace:")
	for _, item := range items {
		fmt.Printf("  %s (%d entries)\n", item.Key, len(item.Value.Data))
	}

	// Use Apply to create or update
	anotherConfigMap := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "feature-config",
			Namespace: "default",
		},
		Data: map[string]string{
			"feature.enabled": "true",
		},
	}

	if err := store.Apply(ctx, "feature-config", anotherConfigMap); err != nil {
		fmt.Println("Apply error:", err)
		return
	}
	fmt.Println("Applied ConfigMap")

	// Delete the ConfigMaps
	if err := store.Delete(ctx, "app-config"); err != nil {
		fmt.Println("Delete error:", err)
		return
	}
	fmt.Println("Deleted app-config")

	if err := store.Delete(ctx, "feature-config"); err != nil {
		fmt.Println("Delete error:", err)
		return
	}
	fmt.Println("Deleted feature-config")

	// Output:
	// Created ConfigMap
	// Retrieved ConfigMap: app-config
	// Data entries: 3
	// Updated ConfigMap
	// ConfigMaps in namespace:
	//   app-config (4 entries)
	// Applied ConfigMap
	// Deleted app-config
	// Deleted feature-config
}
