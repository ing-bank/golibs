package operator

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConditionsDeepCopy(t *testing.T) {
	tests := []struct {
		name string
		in   *Conditions
	}{
		{name: "nil receiver", in: nil},
		{name: "nil slice", in: &Conditions{}},
		{name: "populated", in: &Conditions{Conditions: []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := tt.in.DeepCopy()
			if tt.in == nil {
				if out != nil {
					t.Fatalf("expected nil, got %#v", out)
				}
				return
			}
			if len(out.Conditions) != len(tt.in.Conditions) || (tt.in.Conditions == nil) != (out.Conditions == nil) {
				t.Fatalf("copy mismatch: %#v vs %#v", out, tt.in)
			}
			if len(out.Conditions) > 0 {
				out.Conditions[0].Type = "Changed"
				if tt.in.Conditions[0].Type == "Changed" {
					t.Fatal("copy shares backing array with original")
				}
			}
		})
	}
}
