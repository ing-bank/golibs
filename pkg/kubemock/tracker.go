package kubemock

import (
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/testing"
)

// generationTrackingObjectTracker adds API-server write semantics that the
// client-go tracker lacks: metadata.generation handling and dropping no-op
// writes. Every fake in this package (clientset, controller-runtime client,
// dynamic client) writes through it, so the semantics are identical across them.
type generationTrackingObjectTracker struct {
	testing.ObjectTracker
}

func newGenerationTrackingObjectTracker(scheme *runtime.Scheme) *generationTrackingObjectTracker {
	return &generationTrackingObjectTracker{
		ObjectTracker: testing.NewObjectTracker(scheme, serializer.NewCodecFactory(scheme).UniversalDecoder()),
	}
}

func (t *generationTrackingObjectTracker) Add(obj runtime.Object) error {
	initGeneration(obj)
	return t.ObjectTracker.Add(obj)
}

func (t *generationTrackingObjectTracker) Create(gvr schema.GroupVersionResource, obj runtime.Object, ns string, opts ...metav1.CreateOptions) error {
	initGeneration(obj)
	return t.ObjectTracker.Create(gvr, obj, ns, opts...)
}

func (t *generationTrackingObjectTracker) Update(gvr schema.GroupVersionResource, obj runtime.Object, ns string, opts ...metav1.UpdateOptions) error {
	if t.prepareWrite(gvr, ns, obj) {
		return nil
	}
	return t.ObjectTracker.Update(gvr, obj, ns, opts...)
}

func (t *generationTrackingObjectTracker) Patch(gvr schema.GroupVersionResource, obj runtime.Object, ns string, opts ...metav1.PatchOptions) error {
	if t.prepareWrite(gvr, ns, obj) {
		return nil
	}
	return t.ObjectTracker.Patch(gvr, obj, ns, opts...)
}

// Apply cannot diff against the merged result, so server-side apply always
// increments the generation.
func (t *generationTrackingObjectTracker) Apply(gvr schema.GroupVersionResource, obj runtime.Object, ns string, opts ...metav1.PatchOptions) error {
	if _, current := t.stored(gvr, ns, obj); current != nil {
		accessor, _ := meta.Accessor(obj)
		if accessor.GetGeneration() == 0 {
			accessor.SetGeneration(current.GetGeneration())
		}
	}
	incrementGeneration(obj)
	return t.ObjectTracker.Apply(gvr, obj, ns, opts...)
}

// prepareWrite sets the generation the API server would assign and reports
// whether the write is a no-op. Generation only increments when the desired
// state changes (anything outside metadata and status) or deletion starts.
// A no-op write keeps the stored resourceVersion and must not be persisted:
// otherwise every identical status update would emit a watch event and
// re-trigger reconcilers indefinitely.
func (t *generationTrackingObjectTracker) prepareWrite(gvr schema.GroupVersionResource, ns string, obj runtime.Object) (noop bool) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return false
	}
	currentObj, current := t.stored(gvr, ns, obj)
	if current == nil {
		incrementGeneration(obj)
		return false
	}

	generation := max(current.GetGeneration(), 1)
	deletionStarted := current.GetDeletionTimestamp() == nil && accessor.GetDeletionTimestamp() != nil
	if deletionStarted || !equalIgnoring(currentObj, obj, []string{"apiVersion", "kind", "metadata", "status"}, nil) {
		generation++
	}
	accessor.SetGeneration(generation)

	if !equalIgnoring(currentObj, obj, []string{"apiVersion", "kind"}, []string{"resourceVersion", "managedFields"}) {
		return false
	}
	accessor.SetResourceVersion(current.GetResourceVersion())
	return true
}

// stored returns the tracked object with obj's name, or nils.
func (t *generationTrackingObjectTracker) stored(gvr schema.GroupVersionResource, ns string, obj runtime.Object) (runtime.Object, metav1.Object) {
	accessor, err := meta.Accessor(obj)
	if err != nil || accessor.GetName() == "" {
		return nil, nil
	}
	current, err := t.ObjectTracker.Get(gvr, ns, accessor.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, nil
	}
	currentAccessor, err := meta.Accessor(current)
	if err != nil {
		return nil, nil
	}
	return current, currentAccessor
}

// equalIgnoring compares a and b semantically after removing the given
// top-level and metadata fields. Conversion errors count as a difference.
// Inputs are deep-copied first: ToUnstructured returns an Unstructured's own
// map, and the deletes below must not strip fields from the object being written.
func equalIgnoring(a, b runtime.Object, topLevel, metadata []string) bool {
	ua, errA := runtime.DefaultUnstructuredConverter.ToUnstructured(a.DeepCopyObject())
	ub, errB := runtime.DefaultUnstructuredConverter.ToUnstructured(b.DeepCopyObject())
	if errA != nil || errB != nil {
		return false
	}
	for _, u := range []map[string]any{ua, ub} {
		if md, ok := u["metadata"].(map[string]any); ok {
			for _, field := range metadata {
				delete(md, field)
			}
		}
		for _, field := range topLevel {
			delete(u, field)
		}
	}
	return equality.Semantic.DeepEqual(ua, ub)
}

func initGeneration(obj runtime.Object) {
	if accessor, err := meta.Accessor(obj); err == nil && accessor.GetGeneration() == 0 {
		accessor.SetGeneration(1)
	}
}

func incrementGeneration(obj runtime.Object) {
	if accessor, err := meta.Accessor(obj); err == nil {
		accessor.SetGeneration(accessor.GetGeneration() + 1)
	}
}
