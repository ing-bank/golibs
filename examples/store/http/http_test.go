package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ing-bank/golibs/pkg/ginserver"
	"github.com/ing-bank/golibs/pkg/http"
	"github.com/ing-bank/golibs/pkg/store"
	httpstoreclient "github.com/ing-bank/golibs/pkg/store/backends/http/client"
	httpstoreserver "github.com/ing-bank/golibs/pkg/store/backends/http/server"
	"github.com/ing-bank/golibs/pkg/store/backends/memory"
)

type MyData struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func (e MyData) GetName() string {
	return e.Name
}

func (e MyData) Validate() error {
	if e.Name == "" {
		return fmt.Errorf("value is required")
	}
	return nil
}

type FakeOption string

func (p FakeOption) Serialize() (string, string) {
	return "fakeoption", string(p)
}

var WithFakeOption, _ = store.SerializableOptionBuilder[FakeOption]("fakeoption", func(val string) (store.Option, error) {
	return FakeOption(val), nil
})

func Example() {
	ctx := context.Background()
	cfg := ginserver.DefaultConfig()
	cfg.HTTPServer.Port = 8091

	// Create an in-memory store backend and serve it over HTTP
	backend, _ := httpstoreserver.New[*MyData](memory.NewOrDie[string, *MyData]())
	server, _ := ginserver.NewForConfig(cfg, ginserver.WithRoutes(backend))
	server.RunBackground(ctx)
	time.Sleep(time.Second) // wait for server to start

	// Create an HTTP client to interact with the store server
	url := fmt.Sprintf("http://localhost:%d", cfg.HTTPServer.Port)
	client := httpstoreclient.New[*MyData](url, http.DefaultClient)

	// Create a resource
	obj := &MyData{Name: "example1", Age: 30}
	err := client.Apply(ctx, obj.GetName(), obj)
	if err != nil {
		panic(err)
	}

	// Read the resource back
	readObj, err := client.Read(ctx, obj.GetName())
	if err != nil {
		panic(err)
	}
	fmt.Println("Read object:", readObj.Name, readObj.Age) // Read object: example1 30

	// Perform a dry-run delete (should not actually delete the resource)
	err = client.Delete(ctx, obj.GetName(), store.WithDryRun(true))
	fmt.Println(err) // <nil>

	// Perform delete with an option that cannot be serialized (send over the network)
	err = client.Delete(ctx, obj.GetName(), "FakeOption")
	fmt.Println(err) // unsupported option: option is not serializable

	// Perform delete with an option that can be serialized but is not supported by the store backend
	err = client.Delete(ctx, obj.GetName(), WithFakeOption("foo"))
	// The memory store does not support it and throws an unsupported option error. This is then echoed back to the
	// client via the http server as status: 400, response: {\"error\":\"unsupported option\"}"
	// At the moment the client does not peek into error payloads and uses the status code, so we get:
	fmt.Println(err) // bad request

	// Read the resource back
	readObj, err = client.Read(ctx, obj.GetName())
	if err != nil {
		panic(err)
	}
	fmt.Println("Read object:", readObj.Name, readObj.Age) // Read object: example1 30

	// Output:
	// Read object: example1 30
	// <nil>
	// unsupported option: option is not serializable
	// bad request
	// Read object: example1 30
}
