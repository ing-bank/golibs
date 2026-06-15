// Package reloader provides functionality to watch files and reload Kubernetes deployments when changes are detected.
// Other libraries are being improved to support hot reload of certificates so that a full restart of the application
// is no longer required.
//
// The reloader watches a list of files and when it detects a change, it will update
// the annotations of a specified Kubernetes deployment. This will trigger a rolling
// update of the deployment.
//
// The reloader requires a Kubernetes client and the following RBAC permissions:
//   - get on deployments
//   - update on deployments
//
// Example of a Role and RoleBinding:
//
//	apiVersion: v1
//	kind: ServiceAccount
//	metadata:
//	  name: example-sa
//	  namespace: example-ns
//	---
//	apiVersion: rbac.authorization.k8s.io/v1
//	kind: Role
//	metadata:
//	  name: example-reloader
//	  namespace: example-ns
//	rules:
//	- apiGroups:
//	  - apps
//	  resources:
//	  - deployments
//	  verbs:
//	  - get
//	  - update
//	---
//	apiVersion: rbac.authorization.k8s.io/v1
//	kind: RoleBinding
//	metadata:
//	  name: example-reloader
//	  namespace: example-ns
//	roleRef:
//	  apiGroup: rbac.authorization.k8s.io
//	  kind: Role
//	  name: example-reloader
//	subjects:
//	- kind: ServiceAccount
//	  name: example-sa
//	  namespace: example-ns
package reloader

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/ing-bank/golibs/pkg/config"
	"github.com/ing-bank/golibs/pkg/fsnotify"
	"github.com/ing-bank/golibs/pkg/graceful"
	log "github.com/sirupsen/logrus"
	v12 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// AnnotationPrefix is the prefix for the reloader annotations.
	AnnotationPrefix = "hash"

	// DefaultInterval is the default interval for checking file changes.
	DefaultInterval = time.Minute
)

// Config holds the configuration for the reloader.
type Config struct {
	Namespace  string      `json:"namespace" yaml:"namespace"`
	Deployment string      `json:"deployment" yaml:"deployment"`
	Files      []string    `json:"files" yaml:"files"`
	Interval   v1.Duration `json:"interval" yaml:"interval"`
}

// GetNamespace returns the namespace to watch.
func (c *Config) GetNamespace() (string, error) {
	if c.Namespace != "" {
		return c.Namespace, nil
	}
	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", fmt.Errorf("failed to read namespace from file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// GetDeployment returns the deployment name to watch.
func (c *Config) GetDeployment() (string, error) {
	if c.Deployment != "" {
		return c.Deployment, nil
	}
	if v := strings.TrimSpace(os.Getenv("POD_NAME")); v != "" {
		return v, nil
	}
	hostname := strings.TrimSpace(os.Getenv("HOSTNAME"))
	if hostname == "" {
		return "", fmt.Errorf("failed to get deployment name: HOSTNAME environment variable is not set")
	}

	// Often Pod names look like: <deployment>-<pod-template-hash>-<replica>
	// Example: logging-mux-6cbc494c89-kdmnx
	parts := strings.Split(hostname, "-")
	if len(parts) <= 2 {
		return "", fmt.Errorf("failed to get deployment name from hostname: %s", hostname)
	}
	return strings.Join(parts[:len(parts)-2], "-"), nil
}

// ApplyDefaults applies the default values to the config.
func (c *Config) ApplyDefaults() {
	if c.Interval.Duration == 0 {
		c.Interval = v1.Duration{Duration: DefaultInterval}
	}
}

// Validate validates the config.
func (c *Config) Validate() error {
	if c.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if c.Deployment == "" {
		return fmt.Errorf("deployment is required")
	}
	if len(c.Files) == 0 {
		return fmt.Errorf("at least one file is required")
	}
	return nil
}

// DeploymentReloader is the reloader instance.
type DeploymentReloader struct {
	cfg    *Config
	client kubernetes.Interface
}

// Option is a functional option for configuring the DeploymentReloader.
type Option = config.Option[*DeploymentReloader]

// NewForConfig creates a new DeploymentReloader for the given config.
func NewForConfig(cfg *Config, opts ...Option) (*DeploymentReloader, error) {
	c := *cfg // shallow cfg

	// apply default values to the configuration
	c.ApplyDefaults()

	ns, err := c.GetNamespace()
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace: %w", err)
	}
	c.Namespace = ns

	deploy, err := c.GetDeployment()
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment name: %w", err)
	}
	c.Deployment = deploy

	// validate the configuration
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	kubeconfig, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	kubeclient, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubeclient: %w", err)
	}

	r := &DeploymentReloader{cfg: &c, client: kubeclient}
	if err := config.ApplyOpts(r, opts...); err != nil {
		return nil, fmt.Errorf("failed to apply options: %w", err)
	}

	return r, nil
}

// Run starts the reloader.
func (r *DeploymentReloader) Run(ctx context.Context) error {
	if err := r.Validate(ctx); err != nil {
		return fmt.Errorf("rbac validation failed: %w", err)
	}
	log.WithContext(ctx).Infof("Starting reloader for deployment %s/%s", r.cfg.Namespace, r.cfg.Deployment)
	return graceful.Run(ctx, r.Watch)
}

// RunBackground starts the reloader in the background.
func (r *DeploymentReloader) RunBackground(ctx context.Context) <-chan error {
	return graceful.RunBackground(ctx, r.Run)
}

// dirFS returns the appropriate fs.FS based on the file paths.
func (r *DeploymentReloader) dirFS() fs.FS {
	for _, file := range r.cfg.Files {
		if strings.HasPrefix(file, "/") {
			return os.DirFS("/")
		}
	}
	return os.DirFS(".")
}

// Watch starts watching the files and triggers a reload on change.
func (r *DeploymentReloader) Watch(ctx context.Context) error {
	w := fsnotify.NewIntervalWatcher(r.dirFS(), false)
	return w.Watch(ctx, r.cfg.Files, func(ctx context.Context, hashes map[string][]byte) {
		log.WithContext(ctx).Infof("File changed, getting deploy %s/%s", r.cfg.Namespace, r.cfg.Deployment)
		dep, err := r.client.AppsV1().Deployments(r.cfg.Namespace).Get(ctx, r.cfg.Deployment, v1.GetOptions{})
		if err != nil {
			log.WithContext(ctx).Errorf("Error getting deployment: %v", err)
			return
		}

		for filename, hash := range hashes {
			filenameHasher := sha256.New()
			filenameHasher.Write([]byte(filename))
			filenameHash := filenameHasher.Sum(nil)

			UpdateHashAnnotation(dep, filenameHash, hash)
		}
		log.WithContext(ctx).Infof("Updating annotations")

		_, err = r.client.AppsV1().Deployments(r.cfg.Namespace).Update(ctx, dep, v1.UpdateOptions{})
		if err != nil {
			log.WithContext(ctx).Errorf("Error updating deployment: %v", err)
		}
	}, r.cfg.Interval.Duration)
}

// UpdateHashAnnotation updates the deployment's pod template annotations with the given filename and hash.
func UpdateHashAnnotation(dep *v12.Deployment, filename, hash []byte) {
	if dep.Spec.Template.ObjectMeta.Annotations == nil {
		dep.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}

	key := AnnotationPrefix + "/" + fmt.Sprintf("%x", filename)
	// Annotations have a length limit of 42 characters.
	// We truncate the key to 42 characters, leaving space for the hash.
	if len(key) > 42 {
		key = key[:42]
	}

	dep.Spec.Template.ObjectMeta.Annotations[key] = fmt.Sprintf("%x", hash)
}

// Validate reads and updates (as dryRun) the provided deployment to validate RBAC permissions
func (r *DeploymentReloader) Validate(ctx context.Context) error {
	var nonRootFS, rootFS bool
	for _, file := range r.cfg.Files {
		if strings.HasPrefix(file, "/") {
			rootFS = true
		} else {
			nonRootFS = true
		}
	}
	if rootFS && nonRootFS {
		return fmt.Errorf("cannot watch files from both root fs and non-root fs")
	}

	// try to get the deployment first
	dep, err := r.client.AppsV1().Deployments(r.cfg.Namespace).Get(ctx, r.cfg.Deployment, v1.GetOptions{})
	if err != nil {
		return err
	}

	UpdateHashAnnotation(dep, []byte("reloader-dry-run"), []byte("any"))

	// check if rbac permissions are sufficient by performing a dry-run update
	_, err = r.client.AppsV1().Deployments(r.cfg.Namespace).Update(ctx, dep, v1.UpdateOptions{DryRun: []string{v1.DryRunAll}})
	return err
}
