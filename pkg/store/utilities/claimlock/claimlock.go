// Package claimlock provides a utility to delegate locks without active controllers in a Kubernetes cluster (e.g.
// parent controller got terminated). The main concept is that a lock is assigned to a controller that runs inside a
// Kubernetes cluster (usually this is just a Pod found identified via hostname). The claim lock runner periodically
// checks if the controllers that own the locks are still active in the cluster, and if not, it delegates the locks to
// a new controller. Of course the controller must support functionality to pickup an existing lock (delegation).
//
// The following RBAC is required for claim lock to run:
//
//	apiVersion: rbac.authorization.k8s.io/v1
//	kind: Role
//	metadata:
//	  name: example
//	rules:
//	  - apiGroups: [ "" ]
//	    resources: [ "pods" ]
//	    verbs: [ "list", "get", "watch" ]
//	---
//	apiVersion: rbac.authorization.k8s.io/v1
//	kind: RoleBinding
//	metadata:
//	  name: example
//	roleRef:
//	  apiGroup: rbac.authorization.k8s.io
//	  kind: Role
//	  name: example
//	subjects:
//	  - kind: ServiceAccount
//	    name:  example-sa
//	    namespace: example-dev
//	---
//	apiVersion: v1
//	kind: ServiceAccount
//	metadata:
//	  name: example-sa
package claimlock

import (
	"context"
	"fmt"
	"time"

	"github.com/ing-bank/golibs/pkg/slices"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	corev1i "k8s.io/client-go/kubernetes/typed/core/v1"
)

type Config struct {
	// In case of controller delete events wait to avoid delegating the lock to a replica
	// that will soon disappear. Wait for rolling update to finish. Defaults to 1 minute.
	Latency metav1.Duration `default:"5m"`

	// WatchTimeout is the duration for which the Watch call will be active before it times out. Defaults to 1 hour.
	WatchTimeout int64 `yaml:"watchTimeout"`

	// LabelSelector to filter pods that are considered controllers
	LabelSelector string `yaml:"labelSelector"`
}

func (c *Config) Validate() error {
	c.ApplyDefaults()

	if c.LabelSelector == "" {
		return fmt.Errorf("label selector cannot be empty")
	}

	return nil
}

func (c *Config) ApplyDefaults() {
	if c.Latency.Duration == 0 {
		c.Latency = metav1.Duration{Duration: time.Minute}
	}
	if c.WatchTimeout == 0 {
		c.WatchTimeout = 3600
	}
}

type ClaimLock[V any] struct {
	client  corev1i.PodInterface
	claimer Claimer[V]
	cfg     *Config
}

// New creates a new ClaimLock instance. It requires a Kubernetes client to list and watch Pods, a Claimer to list
// and delegate locks, and a Config to specify the behavior of the ClaimLock. Users must call Run to start ClaimLock.
// Make sure the necessary Kubernetes RBAC permissions are in place for the provided client, otherwise ClaimLock will
// not be able to list and watch Pods.
func New[V any](client corev1i.PodInterface, claimer Claimer[V], cfg *Config) (*ClaimLock[V], error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &ClaimLock[V]{
		client:  client,
		claimer: claimer,
		cfg:     cfg,
	}, nil
}

// Run checks Locks periodically. If a lock is owned by a controller that no longer exists, it delegates the lock to a
// valid controller. It also monitors controller events to react faster to controller deletions. Run will block until
// the provided context is cancelled or an error occurs.
func (c *ClaimLock[V]) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			time.Sleep(c.cfg.Latency.Duration)
		}
		if err := c.CheckLocks(ctx); err != nil {
			return err
		}
		if err := c.monitorEvents(ctx); err != nil {
			return err
		}
	}
}

func (c *ClaimLock[V]) CheckLocks(ctx context.Context) error {
	// Gather known controllers
	pods, err := c.client.List(ctx, metav1.ListOptions{
		LabelSelector: c.cfg.LabelSelector,
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return err
	}

	// Convert list of Pods to map of names for easy lookup (transform to slice of names and then to map)
	controllers := slices.Map(slices.Transform(pods.Items, func(pod corev1.Pod) string {
		log.WithContext(ctx).Infof("found controller %s", pod.Name)
		return pod.Name
	}), func(name string) string { return name })

	// Get the Locks
	locks, err := c.claimer.GetLocks(ctx)
	if err != nil {
		return err
	}

	// Find Locks without owner
	for _, lock := range locks {
		lockOwner, err := c.claimer.GetLockOwner(ctx, lock.Value)
		if err != nil {
			log.WithContext(ctx).Errorf("failed to get lock owner: %v", err)
			return err
		}

		if lockOwner == "" {
			log.WithContext(ctx).Errorf("lock owner is empty, ignoring it: %v", lock)
			continue
		}

		if _, ok := controllers[lockOwner]; !ok {
			log.WithContext(ctx).Infof("lock %s is owned by controller %s that no longer exists", lock.Key, lockOwner)
			if err := c.claimer.ClaimLock(ctx, lock.Key, lock.Value); err != nil {
				log.WithContext(ctx).Errorf("failed to delegate lock %s: %v", lock.Key, err)
				return err
			}
		}
	}

	return nil
}

func (c *ClaimLock[V]) monitorEvents(ctx context.Context) error {
	watcher, err := c.client.Watch(ctx, metav1.ListOptions{
		TimeoutSeconds: &c.cfg.WatchTimeout,
		LabelSelector:  c.cfg.LabelSelector,
	})
	if err != nil {
		return err
	}

	for {
		select {
		case ev, ok := <-watcher.ResultChan():
			if !ok || ev.Type == watch.Deleted {
				return nil
			}
		case <-ctx.Done():
			return nil
		}
	}
}
