package kubemock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNewFakeManager(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	mgr, err := NewManager(nil, ctrl.Options{Scheme: scheme})
	require.NoError(t, err)

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Data: map[string]string{"k": "v"},
	}

	require.NoError(t, mgr.GetClient().Create(context.Background(), cm))

	var stored corev1.ConfigMap
	require.NoError(t, mgr.GetClient().Get(context.Background(), client.ObjectKey{Namespace: "default", Name: "demo"}, &stored))
	require.Equal(t, "demo", stored.Name)
	require.Equal(t, "v", stored.Data["k"])
}

