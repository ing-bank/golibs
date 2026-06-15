package labels

import (
	"testing"

	"github.com/ing-bank/golibs/pkg/store"
	"k8s.io/apimachinery/pkg/labels"
)

func TestLabelsSerialize(t *testing.T) {
	l := Labels{"foo": "bar", "baz": "qux"}
	k, v := l.Serialize()
	if k != "labels" {
		t.Errorf("expected key 'labels', got %s", k)
	}
	parsed := labels.Set(l).String()
	if v != parsed {
		t.Errorf("expected value %s, got %s", parsed, v)
	}
}

func TestWithLabelsOptionRoundTrip(t *testing.T) {
	tests := []Labels{map[string]string{"foo": "bar", "baz": "qux"}}
	for _, tt := range tests {

		// Consumer sets options
		labelsOpt := WithLabels(tt)
		options := []store.Option{labelsOpt}

		// We can parse options back from []any to our expected type
		opt, _ := MatchLabels(&options)

		// Proof the matching works
		if opt["foo"] != "bar" {
			t.Errorf("expected foo=bar, got foo=%s", opt["foo"])
		}

		// We can serialize the option to key/value pairs to send them over the network
		serialized, err := store.SerializeOptions([]store.Option{labelsOpt})
		if err != nil {
			t.Fatalf("SerializeOptions failed: %v", err)
		}

		// The receiving side can unserialize the key/value pairs back to options
		opts, err := store.UnserializeOptions(t.Context(), serialized)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// And we can match our option again
		opt, _ = MatchLabels(&opts)

		// Proof the matching still works
		if opt["foo"] != "bar" {
			t.Errorf("expected foo=bar, got foo=%s", opt["foo"])
		}
	}
}

func TestLabelSelectorSerialize(t *testing.T) {
	sel := LabelSelector("foo=bar")
	k, v := sel.Serialize()
	if k != "labelSelector" || v != "foo=bar" {
		t.Errorf("unexpected serialize result: %s, %s", k, v)
	}
}

func TestLabelSelectorOptionRoundTrip(t *testing.T) {
	foo := WithLabelSelector("foo=bar")
	bar, ok := MatchLabelSelector(&[]store.Option{foo})
	if !ok {
		t.Fatalf("MatchLabelSelector failed to find option")
	}
	if bar != "foo=bar" {
		t.Errorf("expected 'foo=bar', got %v", bar)
	}

	// Serialize
	serialized, err := store.SerializeOptions([]store.Option{foo})
	if err != nil {
		t.Fatalf("SerializeOptions failed: %v", err)
	}
	if serialized["labelSelector"] != "foo=bar" {
		t.Errorf("expected serialized value 'foo=bar', got %v", serialized["labelSelector"])
	}

	// Unserialize
	opts, err := store.UnserializeOptions(t.Context(), serialized)
	if err != nil {
		t.Fatalf("UnserializeOptions failed: %v", err)
	}
	bar2, ok := MatchLabelSelector(&opts)
	if !ok {
		t.Fatalf("MatchLabelSelector failed after unserialize")
	}
	if bar2 != "foo=bar" {
		t.Errorf("expected 'foo=bar' after unserialize, got %v", bar2)
	}
}
