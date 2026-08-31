package kubemock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNewFakeControllerRuntimeClient(t *testing.T) {
	var _ ctrlclient.Client = NewFakeControllerRuntimeClient()

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	c := NewControllerRuntimeClientFromFakeClient(NewFakeClient(), scheme, nil)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Data: map[string]string{"k": "v"},
	}

	require.NoError(t, c.Create(context.Background(), cm))

	var stored corev1.ConfigMap
	require.NoError(t, c.Get(context.Background(), ctrlclient.ObjectKey{Namespace: "default", Name: "demo"}, &stored))
	require.Equal(t, "demo", stored.Name)
	require.Equal(t, "v", stored.Data["k"])

	isNamespaced, err := c.IsObjectNamespaced(&corev1.ConfigMap{})
	require.NoError(t, err)
	require.True(t, isNamespaced)
}
