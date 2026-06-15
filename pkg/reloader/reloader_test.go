package reloader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		c       *Config
		wantErr bool
	}{
		{
			name: "valid",
			c: &Config{
				Namespace:  "ns",
				Deployment: "dp",
				Files:      []string{"file"},
			},
			wantErr: false,
		},
		{
			name: "no namespace",
			c: &Config{
				Deployment: "dp",
				Files:      []string{"file"},
			},
			wantErr: true,
		},
		{
			name: "no deployment",
			c: &Config{
				Namespace: "ns",
				Files:     []string{"file"},
			},
			wantErr: true,
		},
		{
			name: "no files",
			c: &Config{
				Namespace:  "ns",
				Deployment: "dp",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.c.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestUpdateHashAnnotation(t *testing.T) {
	tests := []struct {
		name       string
		dep        *appsv1.Deployment
		filename   []byte
		hash       []byte
		wantAnnos  map[string]string
		wantNewKey bool
	}{
		{
			name: "no annotations",
			dep: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{},
					},
				},
			},
			filename: []byte("file1"),
			hash:     []byte("hash1"),
			wantAnnos: map[string]string{
				"hash/66696c6531": "6861736831",
			},
			wantNewKey: true,
		},
		{
			name: "existing annotations",
			dep: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{"existing": "anno"},
						},
					},
				},
			},
			filename: []byte("file2"),
			hash:     []byte("hash2"),
			wantAnnos: map[string]string{
				"existing":        "anno",
				"hash/66696c6532": "6861736832",
			},
			wantNewKey: true,
		},
		{
			name: "long filename",
			dep: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{},
					},
				},
			},
			filename: []byte("a-very-long-filename-that-will-be-truncated"),
			hash:     []byte("hash3"),
			wantAnnos: map[string]string{
				"hash/612d766572792d6c6f6e672d66696c656e616": "6861736833",
			},
			wantNewKey: true,
		},
		{
			name: "update existing hash",
			dep: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{"hash/66696c6531": "oldhash"},
						},
					},
				},
			},
			filename: []byte("file1"),
			hash:     []byte("newhash"),
			wantAnnos: map[string]string{
				"hash/66696c6531": "6e657768617368",
			},
			wantNewKey: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			UpdateHashAnnotation(tt.dep, tt.filename, tt.hash)
			assert.Equal(t, tt.wantAnnos, tt.dep.Spec.Template.Annotations)
		})
	}
}
