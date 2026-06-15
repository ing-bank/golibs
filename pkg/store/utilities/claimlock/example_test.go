package claimlock

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
	"github.com/ing-bank/golibs/pkg/store/middleware/threadsafe"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var _ Claimer[string] = (*MyClaimer)(nil)

// MyClaimer satisfies the Claimer interface. The Claimer interface is used by the claim-lock runner to list
// and delegate locks. In this example we implement a simple claimer where the locks are stored in a memory map
// and the lock values are the pod names.
type MyClaimer struct {
	db store.Store[string, string]
}

func (m MyClaimer) GetLocks(ctx context.Context) (store.ListItems[string, string], error) {
	return m.db.List(ctx)
}

func (m MyClaimer) GetLockOwner(_ context.Context, value string) (string, error) {
	return value, nil // For our example the value of the lock is the owner, so we just return it
}

func (m MyClaimer) ClaimLock(ctx context.Context, key string, value string) error {
	fmt.Println("Delegating lock", key, "to pod-2")
	return m.db.Apply(ctx, key, "pod-2") // Delegate the lock to a valid controller, we expect pod-2 to exist
}

func Example() {
	ctx := context.Background()

	// Create some fake controller Pods called pod-1 and pod-2, with labels role=controller
	kube := fake.NewSimpleClientset().CoreV1().Pods("example-dev")
	for i := 1; i <= 2; i++ {
		_, _ = kube.Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("pod-%d", i),
			Namespace: "example-dev",
			Labels:    map[string]string{"role": "controller"},
		}}, metav1.CreateOptions{})
	}

	// Our data store, create a few example locks
	db, _ := store.New[string, string](memory.New, threadsafe.New)
	_ = db.Create(ctx, "lock-example-resource", "pod-1") // This lock has a valid controller
	_ = db.Create(ctx, "lock-another-resource", "pod-9") // This lock does not have a valid controller

	// Create a claimer, which will be called by the claim lock runner. A claimer consists of three parts:
	// - a function to list locks
	// - a function to get the owner of a lock
	// - a function to delegate a lock to a new owner.
	claimer := MyClaimer{db: db}

	// Setup claim lock
	checker, err := New(kube, claimer, &Config{LabelSelector: "role=controller", Latency: metav1.Duration{Duration: time.Second}})
	if err != nil {
		log.Fatalf("cannot create claim lock: %v", err)
	}

	// Start a goroutine to simulate pod-1 disappearing after 2 seconds.
	// This should cause the lock "lock-example-resource" to be delegated.
	go func() {
		time.Sleep(2 * time.Second)
		fmt.Println("Deleting pod-1 to simulate it disappearing")
		_ = kube.Delete(ctx, "pod-1", metav1.DeleteOptions{})
	}()

	// Run claim lock, this will block until the context is cancelled.
	// We expect claim lock to find that "lock-example-resource" is owned by a valid controller and do nothing,
	///while "lock-another-resource" is not and delegate it to a valid controller.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := checker.Run(ctx); err != nil {
		log.Fatalf("claimer.Run: %v", err)
	}

	// Output:
	// Delegating lock lock-another-resource to pod-2
	// Deleting pod-1 to simulate it disappearing
	// Delegating lock lock-example-resource to pod-2
}
