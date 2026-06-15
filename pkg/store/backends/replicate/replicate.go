package replicate

import (
	"cmp"
	"context"
	goerrors "errors"
	"fmt"
	"sync"

	"github.com/ing-bank/golibs/pkg/errors"
	"github.com/ing-bank/golibs/pkg/retry"
	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/task/runnable"
	task "github.com/ing-bank/golibs/pkg/task/workflow"
)

var _ store.Store[string, string] = (*Replication[string, string])(nil)

type Config struct {
	WorkflowName string
}

type Replication[K cmp.Ordered, V any] struct {
	stores []store.Store[K, V]
	cfg    Config
}

var ErrRollbackFailed = goerrors.New("rollback failed")

type Rollback chan error

var WithRollback, matchWithRollback = store.OptionBuilder[Rollback]()

func NewBuilder[K cmp.Ordered, V any](cfg Config, storeBuilders ...store.Backend[K, V]) store.Backend[K, V] {
	return func() (store.Store[K, V], error) {
		stores := make([]store.Store[K, V], 0, len(storeBuilders))
		for _, builder := range storeBuilders {
			db, err := builder()
			if err != nil {
				return nil, err
			}
			stores = append(stores, db)
		}
		return New(cfg, stores...), nil
	}
}

func New[K cmp.Ordered, V any](cfg Config, stores ...store.Store[K, V]) store.Store[K, V] {
	return &Replication[K, V]{
		stores: stores,
		cfg:    cfg,
	}
}

func (r *Replication[K, V]) Create(ctx context.Context, key K, value V, opts ...store.Option) error {
	createFunc := func(ctx context.Context, db store.Store[K, V], key K, value V, opts ...store.Option) (V, error) {
		var zero V
		return zero, db.Create(ctx, key, value, opts...)
	}
	rollbackFunc := func(ctx context.Context, db store.Store[K, V], key K, _ V, opts ...store.Option) error {
		return db.Delete(ctx, key, opts...)
	}

	return r.Execute(ctx, key, value, createFunc, rollbackFunc, opts...)
}

func (r *Replication[K, V]) Read(ctx context.Context, key K, opts ...store.Option) (V, error) {
	return r.stores[0].Read(ctx, key, opts...)
}

func (r *Replication[K, V]) Update(ctx context.Context, key K, value V, opts ...store.Option) error {
	createFunc := func(ctx context.Context, db store.Store[K, V], key K, value V, opts ...store.Option) (V, error) {
		var backup V
		var err error
		if _, ok := matchWithRollback(&opts); ok {
			backup, err = db.Read(ctx, key)
			if err != nil {
				return backup, err
			}
		}

		return backup, db.Update(ctx, key, value, opts...)
	}
	rollbackFunc := func(ctx context.Context, db store.Store[K, V], key K, backup V, opts ...store.Option) error {
		return db.Update(ctx, key, backup, opts...)
	}

	return r.Execute(ctx, key, value, createFunc, rollbackFunc, opts...)
}

func (r *Replication[K, V]) Apply(ctx context.Context, key K, value V, opts ...store.Option) error {
	createFunc := func(ctx context.Context, db store.Store[K, V], key K, value V, opts ...store.Option) (V, error) {
		var backup V
		var err error
		if _, ok := matchWithRollback(&opts); ok {
			backup, err = db.Read(ctx, key)
			if err != nil && !goerrors.Is(err, errors.ErrNotFound) {
				return backup, err
			}
		}

		return backup, db.Apply(ctx, key, value, opts...)
	}
	rollbackFunc := func(ctx context.Context, db store.Store[K, V], key K, backup V, opts ...store.Option) error {
		if any(backup) == nil {
			return db.Delete(ctx, key, opts...)
		}
		return db.Update(ctx, key, backup, opts...)
	}

	return r.Execute(ctx, key, value, createFunc, rollbackFunc, opts...)
}

func (r *Replication[K, V]) Delete(ctx context.Context, key K, opts ...store.Option) error {
	createFunc := func(ctx context.Context, db store.Store[K, V], key K, value V, opts ...store.Option) (V, error) {
		var backup V
		var err error
		if _, ok := matchWithRollback(&opts); ok {
			backup, err = db.Read(ctx, key)
			if err != nil && !goerrors.Is(err, errors.ErrNotFound) {
				return backup, err
			}
		}

		return backup, db.Delete(ctx, key, opts...)
	}
	rollbackFunc := func(ctx context.Context, db store.Store[K, V], key K, backup V, opts ...store.Option) error {
		if any(backup) == nil {
			return db.Apply(ctx, key, backup, opts...)
		}
		return nil
	}

	var zero V
	return r.Execute(ctx, key, zero, createFunc, rollbackFunc, opts...)
}

func (r *Replication[K, V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[K, V], error) {
	return r.stores[0].List(ctx, opts...)
}

func (r *Replication[K, V]) Execute(ctx context.Context, key K, value V,
	do func(ctx context.Context, db store.Store[K, V], key K, value V, opts ...store.Option) (V, error),
	rollback func(ctx context.Context, db store.Store[K, V], key K, value V, opts ...store.Option) error,
	opts ...store.Option,
) error {
	type Result struct {
		Err   error
		Value V
	}
	errCh, useRollback := matchWithRollback(&opts)

	didLock := sync.Map{}
	errs := task.NewWorkflowFor(r.cfg.WorkflowName, r.stores, func(ctx context.Context, db store.Store[K, V], _ V) error {
		backup, err := do(ctx, db, key, value, opts...)
		if err != nil {
			didLock.Store(db, &Result{Err: err, Value: backup})
		}
		return err
	}).WithRetryPolicy(retry.RetryOnce).ExecuteConcurrent(ctx, value) // Blocking call

	if useRollback && runnable.AnyError(errs) {
		rollbackErrs := task.NewWorkflowFor(r.cfg.WorkflowName, r.stores, func(ctx context.Context, db store.Store[K, V], _ V) error {
			if result, found := didLock.Load(db); found { // There was no err in locking DB, so rollback for this DB is needed
				return rollback(ctx, db, key, result.(*Result).Value, opts...)
			}
			return nil // Locking had err (failed), no need to Unlock
		}).ExecuteConcurrent(ctx, value)

		for _, err := range rollbackErrs {
			if err != nil {
				errCh <- fmt.Errorf("%w: %w", ErrRollbackFailed, err)
			}
		}
	}

	return goerrors.Join(errs...)
}
