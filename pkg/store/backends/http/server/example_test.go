package server

import (
	"context"
	"net/http/httptest"

	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/http"
	"github.com/ing-bank/golibs/pkg/store"
	labelstore "github.com/ing-bank/golibs/pkg/store/backends/labels"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
)

// ExampleType is a sample type implementing Nameable.
type ExampleType struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (e ExampleType) GetName() string { return e.Name }

func (e ExampleType) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("name is required")
	}
	if e.Value == "" {
		return fmt.Errorf("value is required")
	}
	return nil
}

func Example() {
	gin.SetMode(gin.TestMode)
	memStore, _ := store.New[string, ExampleType](memory.New)
	srv, _ := New[ExampleType](memStore, &Config{ResourceVersion: "v1", PluralResourceName: "examples"})
	r := gin.New()
	srv.Register(r)

	ts := httptest.NewServer(r)
	defer ts.Close()
	client := http.DefaultClient

	ctx := context.Background()

	// Create an item
	item := ExampleType{Name: "foo", Value: "bar"}
	resp := client.Post(ctx, ts.URL+"/v1/examples", item)
	fmt.Println("Create status:", resp.Status)

	// Provide an unsupported option. This action will fail as the memory store does not support fake option.
	resp = client.Delete(ctx, ts.URL+"/v1/examples/foo?fake=true", nil)
	fmt.Println("Delete status:", resp.Status)
	fmt.Println("Delete body:", string(resp.Raw))

	// Read the item
	var got ExampleType
	resp = client.Get(ctx, ts.URL+"/v1/examples/foo").Parse(&got)
	fmt.Println("Read status:", resp.Status)
	fmt.Println("Read value:", got)

	// Output:
	// Create status: 201
	// Delete status: 400
	// Delete body: {"error":"unsupported option: cannot match deserializer for key 'fake'"}
	// Read status: 200
	// Read value: {foo bar}
}

func Example_http() { // Example with Labels
	gin.SetMode(gin.TestMode)
	db := memory.NewOrDie[string, *labelstore.LabeledData[string, ExampleType]]()
	memStore, _ := store.New[string, ExampleType](labelstore.NewBackend[string, ExampleType](db, labelstore.Config[ExampleType]{}))
	srv, _ := New[ExampleType](memStore, &Config{ResourceVersion: "v1", PluralResourceName: "examples", UseLabels: true})
	r := gin.New()
	srv.Register(r)

	ts := httptest.NewServer(r)
	defer ts.Close()
	client := http.DefaultClient

	ctx := context.Background()

	// Create an item
	item := ExampleType{Name: "foo", Value: "bar"}
	resp := client.Post(ctx, ts.URL+"/v1/examples", item)
	fmt.Println("Create status:", resp.Status)

	// Provide an unsupported option. This action will fail as the memory store does not support fake option.
	resp = client.Delete(ctx, ts.URL+"/v1/examples/foo?fake=true", nil)
	fmt.Println("Delete status:", resp.Status)
	fmt.Println("Delete body:", string(resp.Raw))

	// Read the item
	var got ExampleType
	resp = client.Get(ctx, ts.URL+"/v1/examples/foo").Parse(&got)
	fmt.Println("Read status:", resp.Status)
	fmt.Println("Read value:", got)

	// Show the raw LabeledData in the memory store, showing that the server is setting labels
	raw, _ := db.Read(ctx, "foo")
	fmt.Println("raw value:", raw)
	_ = db.Apply(ctx, "junk", &labelstore.LabeledData[string, ExampleType]{}) // Should be filtered out by labels

	var items []store.ListItem[string, ExampleType]
	resp = client.Get(ctx, ts.URL+"/v1/examples").Parse(&items)
	fmt.Println("items:", items)

	// Output:
	// Create status: 201
	// Delete status: 400
	// Delete body: {"error":"unsupported option: cannot match deserializer for key 'fake'"}
	// Read status: 200
	// Read value: {foo bar}
	// raw value: &{foo map[pluralResourceName:examples resourceVersion:v1] {foo bar}}
	// items: [{foo {foo bar}}]
}
