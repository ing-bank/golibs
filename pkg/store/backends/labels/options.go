package labels

import (
	"github.com/ing-bank/golibs/pkg/store"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// SupportedOptions is a list of options that are supported by the ConfigMap store.
// This is used for documentation purposes.
var SupportedOptions = []store.Option{
	WithLabelSelector,
	WithLabels,
}

type LabelSelector string

func (l LabelSelector) Serialize() (string, string) {
	return "labelSelector", string(l)
}

var WithLabelSelector, MatchLabelSelector = store.SerializableStringOptionBuilder("labelSelector")

type Labels map[string]string // labels.Set is type map[string]string

func (l Labels) Serialize() (string, string) {
	return "labels", labels.Set(l).String()
}

// MatchLabels should only be used by backends, not by clients
var WithLabels, MatchLabels = store.SerializableMapStringOfStringsBuilder("labels")

// WithLabel is syntactic sugar for WithLabels for the common case of adding a single label.
// It is exactly equivalent to calling WithLabels with a map of one entry.
func WithLabel(key, value string) store.Option {
	return WithLabels(map[string]string{key: value})
}

// WithMapLabelSelector converts a map into a label selector. Errors are ignored.
var WithMapLabelSelector = func(match map[string]string) store.Option {
	raw := metav1.LabelSelector{MatchLabels: match}
	sel, _ := metav1.LabelSelectorAsSelector(&raw)
	return LabelSelector(sel.String())
}

var WithSingleLabelSelector = func(key, value string) store.Option {
	return WithMapLabelSelector(map[string]string{key: value})
}
