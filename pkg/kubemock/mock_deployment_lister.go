package kubemock

import (
	"context"

	appsV1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/fake"
	appsV1Lister "k8s.io/client-go/listers/apps/v1"
)

// MockDeploymentNamespaceLister implements List and Get for Deployments in a namespace
type MockDeploymentNamespaceLister struct {
	client    *fake.Clientset
	namespace string
}

func NewMockDeploymentNamespaceLister(client *fake.Clientset, namespace string) *MockDeploymentNamespaceLister {
	return &MockDeploymentNamespaceLister{client: client, namespace: namespace}
}

func (m *MockDeploymentNamespaceLister) List(selector labels.Selector) ([]*appsV1.Deployment, error) {
	opts := metav1.ListOptions{LabelSelector: selector.String()}
	list, err := m.client.AppsV1().Deployments(m.namespace).List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}
	var result []*appsV1.Deployment
	for i := range list.Items {
		item := &list.Items[i]
		result = append(result, item)
	}
	return result, nil
}

func (m *MockDeploymentNamespaceLister) Get(name string) (*appsV1.Deployment, error) {
	return m.client.AppsV1().Deployments(m.namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

// MockDeploymentLister implements List and Deployments(namespace string) using the fake Client
type MockDeploymentLister struct {
	Client *fake.Clientset
}

func (m *MockDeploymentLister) List(selector labels.Selector) ([]*appsV1.Deployment, error) {
	opts := metav1.ListOptions{LabelSelector: selector.String()}
	list, err := m.Client.AppsV1().Deployments("").List(context.TODO(), opts)
	if err != nil {
		return nil, err
	}
	var result []*appsV1.Deployment
	for i := range list.Items {
		item := &list.Items[i]
		result = append(result, item)
	}
	return result, nil
}

func (m *MockDeploymentLister) Deployments(namespace string) appsV1Lister.DeploymentNamespaceLister {
	return &MockDeploymentNamespaceLister{client: m.Client, namespace: namespace}
}
