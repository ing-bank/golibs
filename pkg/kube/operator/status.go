package operator

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type StatusConditions interface {
	GetConditions() *[]metav1.Condition
}

type Conditions struct {
	Conditions []metav1.Condition `json:"conditions"`
}

func (s *Conditions) GetConditions() *[]metav1.Condition {
	return &s.Conditions
}
