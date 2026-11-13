package operator

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// StatusConditions is implemented by resources whose status exposes a mutable
// list of metav1.Condition values managed by the reconciler.
type StatusConditions interface {
	GetConditions() *[]metav1.Condition
}

// Conditions is an embeddable status fragment that satisfies StatusConditions.
// It provides DeepCopyInto/DeepCopy so that controller-gen generated deepcopy
// code for structs embedding it compiles, since controller-gen does not
// generate methods for types outside the client's API package.
type Conditions struct {
	Conditions []metav1.Condition `json:"conditions"`
}

// GetConditions returns a pointer to the condition slice so callers can
// mutate it in place (e.g. via meta.SetStatusCondition).
func (s *Conditions) GetConditions() *[]metav1.Condition {
	return &s.Conditions
}

// DeepCopyInto copies the receiver into out. in must be non-nil.
func (s *Conditions) DeepCopyInto(out *Conditions) {
	*out = *s
	if s.Conditions != nil {
		in, out := &s.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy returns a deep copy of the receiver, or nil if the receiver is nil.
func (s *Conditions) DeepCopy() *Conditions {
	if s == nil {
		return nil
	}
	out := new(Conditions)
	s.DeepCopyInto(out)
	return out
}
