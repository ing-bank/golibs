package kubemock

import (
	"context"

	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/fake"
)

// MockNamespaceLister implements List and Get for Namespaces using the fake Client
type MockNamespaceLister struct {
	Client *fake.Clientset
}

func NewMockNamespaceLister(client *fake.Clientset) *MockNamespaceLister {
	return &MockNamespaceLister{Client: client}
}

func (m *MockNamespaceLister) List(selector labels.Selector) ([]*coreV1.Namespace, error) {
	opts := metav1.ListOptions{LabelSelector: selector.String()}
	list, err := m.Client.CoreV1().Namespaces().List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}
	var result []*coreV1.Namespace
	for i := range list.Items {
		item := &list.Items[i]
		result = append(result, item)
	}
	return result, nil
}

func (m *MockNamespaceLister) Get(name string) (*coreV1.Namespace, error) {
	return m.Client.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
}
