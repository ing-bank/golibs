package kubemock

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/fake"
	v1l "k8s.io/client-go/listers/core/v1"
)

// MockConfigMapNamespaceLister implements List and Get for a Namespace using the fake client
// This is modeled after the NetworkPolicy mock lister
// It is useful for unit tests and mock scenarios

type MockConfigMapNamespaceLister struct {
	Client    *fake.Clientset
	Namespace string
}

func NewMockConfigMapNamespaceLister(client *fake.Clientset, namespace string) *MockConfigMapNamespaceLister {
	return &MockConfigMapNamespaceLister{Client: client, Namespace: namespace}
}

func (m *MockConfigMapNamespaceLister) List(selector labels.Selector) ([]*corev1.ConfigMap, error) {
	opts := metav1.ListOptions{LabelSelector: selector.String()}
	list, err := m.Client.CoreV1().ConfigMaps(m.Namespace).List(context.Background(), opts)
	if err != nil {
		return nil, err
	}
	var result []*corev1.ConfigMap
	for i := range list.Items {
		item := &list.Items[i]
		result = append(result, item)
	}
	return result, nil
}

func (m *MockConfigMapNamespaceLister) Get(name string) (*corev1.ConfigMap, error) {
	return m.Client.CoreV1().ConfigMaps(m.Namespace).Get(context.Background(), name, metav1.GetOptions{})
}

// MockConfigMapLister implements ConfigMaps(Namespace string) and List

type MockConfigMapLister struct {
	Client *fake.Clientset
}

func NewMockConfigMapLister(client *fake.Clientset) *MockConfigMapLister {
	return &MockConfigMapLister{Client: client}
}

func (m *MockConfigMapLister) ConfigMaps(namespace string) v1l.ConfigMapNamespaceLister {
	return &MockConfigMapNamespaceLister{Client: m.Client, Namespace: namespace}
}

func (m *MockConfigMapLister) List(selector labels.Selector) ([]*corev1.ConfigMap, error) {
	opts := metav1.ListOptions{LabelSelector: selector.String()}
	list, err := m.Client.CoreV1().ConfigMaps("").List(context.Background(), opts)
	if err != nil {
		return nil, err
	}
	var result []*corev1.ConfigMap
	for i := range list.Items {
		item := &list.Items[i]
		result = append(result, item)
	}
	return result, nil
}
