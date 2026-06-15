package replicate

import (
	"context"
	goerrors "errors"
	"testing"

	"github.com/ing-bank/golibs/pkg/errors"
	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
)

func newReplication() *Replication[string, string] {
	mem1, _ := memory.New[string, string]()
	mem2, _ := memory.New[string, string]()
	return &Replication[string, string]{
		stores: []store.Store[string, string]{mem1, mem2},
		cfg:    Config{WorkflowName: "test"},
	}
}

func TestApplyAndRead(t *testing.T) {
	t.Parallel()
	r := newReplication()
	err := r.Apply(t.Context(), "a", "1")
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	val, err := r.Read(t.Context(), "a")
	if err != nil || val != "1" {
		t.Fatalf("Read failed: got %v, err %v", val, err)
	}
}

func TestUpdate(t *testing.T) {
	t.Parallel()
	r := newReplication()
	_ = r.Apply(t.Context(), "a", "1")
	err := r.Update(t.Context(), "a", "2")
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	val, err := r.Read(t.Context(), "a")
	if err != nil || val != "2" {
		t.Fatalf("Update failed: got %v, err %v", val, err)
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()
	r := newReplication()
	_ = r.Apply(t.Context(), "a", "1")
	err := r.Delete(t.Context(), "a")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err = r.Read(t.Context(), "a")
	if err == nil {
		t.Fatalf("Expected error for deleted key")
	}
}

func TestList(t *testing.T) {
	t.Parallel()
	r := newReplication()
	_ = r.Apply(t.Context(), "a", "1")
	_ = r.Apply(t.Context(), "b", "2")
	items, err := r.List(t.Context())
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("List count mismatch: got %d, want 2", len(items))
	}
	keys := map[string]bool{}
	for _, item := range items {
		keys[item.Key] = true
	}
	if !keys["a"] || !keys["b"] {
		t.Fatalf("List missing keys: %v", keys)
	}
}

func TestRollbackOnError(t *testing.T) {
	t.Parallel()
	r := newReplication()
	// Inject a store that always fails on update
	badStore := &badStoreMock{}
	r.stores = append(r.stores, badStore)
	_ = r.Apply(t.Context(), "a", "1")

	rollbackChan := make(chan error, 10)
	err := r.Update(t.Context(), "a", "fail", WithRollback(rollbackChan))

	if err == nil {
		t.Fatalf("Expected error from bad store")
	}
	select {
	case rollbackErr := <-rollbackChan:
		if !goerrors.Is(rollbackErr, ErrRollbackFailed) {
			t.Fatalf("Expected ErrRollbackFailed, got %v", rollbackErr)
		}
	default:
		t.Fatalf("Expected rollback error")
	}
}

type badStoreMock struct{}

func (b *badStoreMock) Create(ctx context.Context, key string, value string, opts ...store.Option) error {
	return goerrors.New("fail")
}
func (b *badStoreMock) Read(ctx context.Context, key string, opts ...store.Option) (string, error) {
	return "", errors.ErrNotFound
}
func (b *badStoreMock) Update(ctx context.Context, key string, value string, opts ...store.Option) error {
	return goerrors.New("fail")
}
func (b *badStoreMock) Apply(ctx context.Context, key string, value string, opts ...store.Option) error {
	return goerrors.New("fail")
}
func (b *badStoreMock) Delete(ctx context.Context, key string, opts ...store.Option) error {
	return goerrors.New("fail")
}
func (b *badStoreMock) List(ctx context.Context, opts ...store.Option) (store.ListItems[string, string], error) {
	return nil, goerrors.New("fail")
}
