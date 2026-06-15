package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ing-bank/golibs/pkg/store/backends/kubernetes"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

func Example_cached() {
	ctx := context.Background()

	// Configure the Kubernetes backend for ConfigMaps
	cfg := kubernetes.CachedConfig{
		Config: kubernetes.Config{
			Namespace: "default",
			Group:     "",
			Version:   "v1",
			Resource:  "configmaps",
		},
	}

	// Create a CACHED store that uses informers (like operators)
	// Using NewCachedFake for this example (doesn't require a real cluster)
	// For production use: store, err := kubernetes.NewCachedForConfig[*v1.ConfigMap](cfg)
	store := kubernetes.NewCachedFake[*v1.ConfigMap](cfg)
	defer store.Stop()

	// Add event handlers to react to changes in real-time
	// Note: Event handlers receive *unstructured.Unstructured, not typed objects
	_, _ = store.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			unstructuredObj := obj.(*unstructured.Unstructured)
			fmt.Printf("ConfigMap added: %s\n", unstructuredObj.GetName())
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			unstructuredObj := newObj.(*unstructured.Unstructured)
			fmt.Printf("ConfigMap updated: %s\n", unstructuredObj.GetName())
		},
		DeleteFunc: func(obj interface{}) {
			unstructuredObj := obj.(*unstructured.Unstructured)
			fmt.Printf("ConfigMap deleted: %s\n", unstructuredObj.GetName())
		},
	})

	// Create a ConfigMap using typed v1.ConfigMap
	configMap := &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cached-example",
			Namespace: "default",
		},
		Data: map[string]string{
			"key": "value",
		},
	}

	// Create goes to API server
	if err := store.Create(ctx, "cached-example", configMap); err != nil {
		fmt.Println("Create error:", err)
		return
	}
	fmt.Println("Created ConfigMap (via API)")

	// Give the informer a moment to update the cache
	time.Sleep(100 * time.Millisecond)

	// Read from CACHE (no API call!)
	retrieved, err := store.Read(ctx, "cached-example")
	if err != nil {
		fmt.Println("Read error:", err)
		return
	}
	fmt.Printf("Retrieved from cache: %s (no API call!)\n", retrieved.Name)

	// List from CACHE (no API call!)
	items, err := store.List(ctx)
	if err != nil {
		fmt.Println("List error:", err)
		return
	}
	fmt.Printf("Listed %d ConfigMaps from cache (no API call!)\n", len(items))

	// Update goes to API server, but cache is automatically updated via watch
	retrieved.Data["updated"] = "true"
	if err := store.Update(ctx, "cached-example", retrieved); err != nil {
		fmt.Println("Update error:", err)
		return
	}
	fmt.Println("Updated ConfigMap (via API)")

	// Give the informer a moment to update the cache
	time.Sleep(100 * time.Millisecond)

	// Read updated value from cache
	updated, err := store.Read(ctx, "cached-example")
	if err != nil {
		fmt.Println("Read error:", err)
		return
	}
	fmt.Printf("Cache automatically updated: updated=%s\n", updated.Data["updated"])

	// Delete
	if err := store.Delete(ctx, "cached-example"); err != nil {
		fmt.Println("Delete error:", err)
		return
	}
	fmt.Println("Deleted ConfigMap")

	// Give time for delete event to process
	time.Sleep(100 * time.Millisecond)

	// Output:
	// Created ConfigMap (via API)
	// ConfigMap added: cached-example
	// Retrieved from cache: cached-example (no API call!)
	// Listed 1 ConfigMaps from cache (no API call!)
	// Updated ConfigMap (via API)
	// ConfigMap updated: cached-example
	// Cache automatically updated: updated=true
	// Deleted ConfigMap
	// ConfigMap deleted: cached-example
}
