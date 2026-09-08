package client

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	httputil "github.com/ing-bank/golibs/pkg/http"
	"github.com/ing-bank/golibs/pkg/store"
	httpserver "github.com/ing-bank/golibs/pkg/store/backends/http/server"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
	"github.com/stretchr/testify/assert"
)

type clientTestType struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Value     string `json:"value"`
}

func (t clientTestType) GetName() string { return t.Namespace + "/" + t.Name }
func (t clientTestType) Validate() error { return nil }

func TestClient_ReadWriteNamespacedKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memStore, _ := store.New[string, clientTestType](memory.New)
	s, err := httpserver.NewForConfig[clientTestType](memStore, httpserver.Config{
		ResourceVersion:    "v1",
		PluralResourceName: "testtypes",
		KeyDeserializer:    httpserver.KeyDeserializerBase64,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	router := gin.New()
	s.Register(router)
	ts := httptest.NewServer(router)
	defer ts.Close()

	c, err := httputil.NewClient()
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	c2, err := NewForConfig[clientTestType](ts.URL+"/v1/testtypes", c, Config{
		KeySerializer: KeySerializerBase64,
	})
	if err != nil {
		t.Fatalf("new client for config: %v", err)
	}

	err = c2.Create(context.Background(), "team-a/demo", clientTestType{Namespace: "team-a", Name: "demo", Value: "ok"})
	assert.NoError(t, err)

	got, err := c2.Read(context.Background(), "team-a/demo")
	assert.NoError(t, err)
	assert.Equal(t, "demo", got.Name)
	assert.Equal(t, "ok", got.Value)

	err = c2.Update(context.Background(), "team-a/demo", clientTestType{Namespace: "team-a", Name: "demo", Value: "updated"})
	assert.NoError(t, err)

	got, err = c2.Read(context.Background(), "team-a/demo")
	assert.NoError(t, err)
	assert.Equal(t, "updated", got.Value)
}
