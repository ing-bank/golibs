package main

import (
	"context"
	"fmt"

	"github.com/ing-bank/golibs/pkg/kubemock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Example shows the smallest setup: a client-go kubernetes.Interface backed by
// memory. Use it for code that takes a clientset and doesn't need a manager.
func Example() {
	ctx := context.Background()
	cs := kubemock.NewFakeClient()
	cms := cs.CoreV1().ConfigMaps("default")

	cm, _ := cms.Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "app"},
		Data:       map[string]string{"color": "blue"},
	}, metav1.CreateOptions{})
	fmt.Println("created, generation:", cm.Generation)

	// Like the API server, generation only changes when the desired state changes.
	cm.Labels = map[string]string{"team": "a"}
	cm, _ = cms.Update(ctx, cm, metav1.UpdateOptions{})
	fmt.Println("label added, generation:", cm.Generation)

	cm.Data["color"] = "green"
	cm, _ = cms.Update(ctx, cm, metav1.UpdateOptions{})
	fmt.Println("data changed, generation:", cm.Generation)

	// Dry-run writes are validated but not persisted.
	_, err := cms.Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "preview"}},
		metav1.CreateOptions{DryRun: kubemock.DryRunOption(true)})
	_, getErr := cms.Get(ctx, "preview", metav1.GetOptions{})
	fmt.Println("dry-run create error:", err, "| stored:", getErr == nil)

	// Output:
	// created, generation: 1
	// label added, generation: 1
	// data changed, generation: 2
	// dry-run create error: <nil> | stored: false
}
