package server

import (
	"encoding/base64"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	httputil "github.com/ing-bank/golibs/pkg/http"
	"github.com/ing-bank/golibs/pkg/store"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
	"github.com/stretchr/testify/assert"
)

// testType is a simple implementation of Nameable for testing.
type testType struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Value     string `json:"value"`
}

func (t testType) GetName() string {
	if t.Namespace == "" {
		return t.Name
	}

	return strings.Join([]string{t.Namespace, t.Name}, "/")
}

func (t testType) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("value is required")
	}
	if t.Value == "" {
		return fmt.Errorf("value is required")
	}
	return nil
}

func TestServerHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memStore, _ := store.New[string, testType](memory.New)
	server, _ := New[testType](memStore)
	router := gin.New()
	server.Register(router)

	ts := httptest.NewServer(router)
	defer ts.Close()
	client, _ := httputil.NewClient()

	// Create
	body := testType{Name: "foo", Value: "bar"}
	resp := client.Post(t.Context(), ts.URL+"/", body)
	assert.Equal(t, 201, resp.Status)

	// Read
	var got testType
	resp = client.Get(t.Context(), ts.URL+"/foo").Parse(&got)
	assert.Equal(t, 200, resp.Status)
	assert.Equal(t, body, got)

	// Update
	body.Value = "baz"
	resp = client.Put(t.Context(), ts.URL+"/foo", body)
	assert.Equal(t, 200, resp.Status)
	val, _ := memStore.Read(t.Context(), "foo")
	assert.Equal(t, "baz", val.Value)

	// List
	var items []store.ListItem[string, testType]
	resp = client.Get(t.Context(), ts.URL+"/").Parse(&items)
	assert.Equal(t, 200, resp.Status)
	assert.Len(t, items, 1)
	assert.Equal(t, "foo", items[0].Key)

	// Apply (name match)
	body.Value = "applied"
	resp = client.Post(t.Context(), ts.URL+"/foo", body)
	assert.Equal(t, 200, resp.Status)
	val, _ = memStore.Read(t.Context(), "foo")
	assert.Equal(t, "applied", val.Value)

	// Apply (name mismatch)
	badBody := testType{Name: "bar", Value: "fail"}
	resp = client.Post(t.Context(), ts.URL+"/foo", badBody)
	assert.Equal(t, 400, resp.Status)

	// Delete
	resp = client.Delete(t.Context(), ts.URL+"/foo", nil)
	assert.Equal(t, 204, resp.Status)
	_, err := memStore.Read(t.Context(), "foo")
	assert.Error(t, err)

	// Read after delete
	resp = client.Get(t.Context(), ts.URL+"/foo")
	assert.Equal(t, 404, resp.Status)
}

func TestServerHandlers_NamespacedKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memStore, _ := store.New[string, testType](memory.New)
	server, _ := NewForConfig[testType](memStore, Config{
		ResourceVersion:    "v1",
		PluralResourceName: "testtypes",
		KeyDeserializer:    KeyDeserializerBase64,
	})
	router := gin.New()
	server.Register(router)

	ts := httptest.NewServer(router)
	defer ts.Close()
	client, _ := httputil.NewClient()

	body := testType{Namespace: "team-a", Name: "demo", Value: "ok"}
	resp := client.Post(t.Context(), ts.URL+"/v1/testtypes", body)
	assert.Equal(t, 201, resp.Status)

	var got testType
	resp = client.Get(t.Context(), ts.URL+"/v1/testtypes/"+base64.RawURLEncoding.EncodeToString([]byte("team-a/demo"))).Parse(&got)
	assert.Equal(t, 200, resp.Status)
	assert.Equal(t, "demo", got.Name)
	assert.Equal(t, "ok", got.Value)

	body.Value = "updated"
	resp = client.Put(t.Context(), ts.URL+"/v1/testtypes/"+base64.RawURLEncoding.EncodeToString([]byte("team-a/demo")), body)
	assert.Equal(t, 200, resp.Status)

	resp = client.Delete(t.Context(), ts.URL+"/v1/testtypes/"+base64.RawURLEncoding.EncodeToString([]byte("team-a/demo")), nil)
	assert.Equal(t, 204, resp.Status)
}
