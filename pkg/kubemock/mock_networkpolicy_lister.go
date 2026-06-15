package kubemock

import (
	"context"

	k8snetv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/fake"
	v1l "k8s.io/client-go/listers/networking/v1"
)

// MockNetworkPolicyNamespaceLister implements List and Get for a Namespace using the fake client
// This is moved from the test files for reuse
type MockNetworkPolicyNamespaceLister struct {
	Client    *fake.Clientset
	Namespace string
}

func NewMockNetworkPolicyNamespaceLister(client *fake.Clientset, namespace string) *MockNetworkPolicyNamespaceLister {
	return &MockNetworkPolicyNamespaceLister{Client: client, Namespace: namespace}
}

func (m *MockNetworkPolicyNamespaceLister) List(selector labels.Selector) ([]*k8snetv1.NetworkPolicy, error) {
	opts := metav1.ListOptions{LabelSelector: selector.String()}
	list, err := m.Client.NetworkingV1().NetworkPolicies(m.Namespace).List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}
	var result []*k8snetv1.NetworkPolicy
	for i := range list.Items {
		item := &list.Items[i]
		result = append(result, item)
	}
	return result, nil
}

func (m *MockNetworkPolicyNamespaceLister) Get(name string) (*k8snetv1.NetworkPolicy, error) {
	return m.Client.NetworkingV1().NetworkPolicies(m.Namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

// MockNetworkPolicyLister implements NetworkPolicies(Namespace string) and List
type MockNetworkPolicyLister struct {
	Client *fake.Clientset
}

func (m *MockNetworkPolicyLister) NetworkPolicies(namespace string) v1l.NetworkPolicyNamespaceLister {
	return &MockNetworkPolicyNamespaceLister{Client: m.Client, Namespace: namespace}
}

func (m *MockNetworkPolicyLister) List(selector labels.Selector) ([]*k8snetv1.NetworkPolicy, error) {
	opts := metav1.ListOptions{LabelSelector: selector.String()}
	list, err := m.Client.NetworkingV1().NetworkPolicies("").List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}
	var result []*k8snetv1.NetworkPolicy
	for i := range list.Items {
		item := &list.Items[i]
		result = append(result, item)
	}
	return result, nil
}
