package client

import (
	"context"
	"fmt"
	"net/http"

	httputil "github.com/ing-bank/golibs/pkg/http"
	"github.com/ing-bank/golibs/pkg/opt"
	"github.com/ing-bank/golibs/pkg/store"
	httpstore "github.com/ing-bank/golibs/pkg/store/backends/http"
)

// Client implements store.Store using HTTP requests to a remote store server.
type Client[V httpstore.ValidatableNameable] struct {
	url  string
	http *httputil.Client
	cfg  Config
}

type Config struct {
	OptionSerializer func(opts []store.Option) (store.SerializedOptions, error)
}

// New creates a new HTTP store client for the given base URL and custom http client.
func New[V httpstore.ValidatableNameable](baseURL string, httpClient *httputil.Client, optCfg ...Config) *Client[V] {
	cfg := opt.Opt(Config{OptionSerializer: store.SerializeOptions}, optCfg)
	return &Client[V]{
		url:  baseURL,
		http: httpClient,
		cfg:  cfg,
	}
}

func NewBackend[V httpstore.ValidatableNameable](baseURL string, httpClient *httputil.Client) store.Backend[string, V] {
	return func() (store.Store[string, V], error) {
		return New[V](baseURL, httpClient), nil
	}
}

func (c *Client[V]) Create(ctx context.Context, key string, value V, opts ...store.Option) error {
	if key != value.GetName() {
		return fmt.Errorf("key %q does not match value name %q", key, value.GetName())
	}
	serializedOptions, err := c.cfg.OptionSerializer(opts)
	if err != nil {
		return err
	}

	resp := c.http.Post(ctx, c.url, value,
		httputil.WithRawQuery(serializedOptions.AsQuery()),
	)
	if resp.Status == http.StatusCreated {
		return nil
	}
	return resp.Error()
}

func (c *Client[V]) Read(ctx context.Context, key string, opts ...store.Option) (V, error) {
	var out V
	serializedOptions, err := c.cfg.OptionSerializer(opts)
	if err != nil {
		return out, err
	}

	url := fmt.Sprintf("%s/%v", c.url, key)
	resp := c.http.Get(ctx, url, httputil.WithRawQuery(serializedOptions.AsQuery())).Parse(&out)
	return out, resp.Error()
}

func (c *Client[V]) Update(ctx context.Context, key string, value V, opts ...store.Option) error {
	if key != value.GetName() {
		return fmt.Errorf("key %q does not match value name %q", key, value.GetName())
	}
	serializedOptions, err := c.cfg.OptionSerializer(opts)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/%v", c.url, key)
	resp := c.http.Put(ctx, url, value,
		httputil.WithRawQuery(serializedOptions.AsQuery()),
	)
	if resp.Status == http.StatusOK {
		return nil
	}
	return resp.Error()
}

func (c *Client[V]) Apply(ctx context.Context, key string, value V, opts ...store.Option) error {
	if key != value.GetName() {
		return fmt.Errorf("key %q does not match value name %q", key, value.GetName())
	}
	serializedOptions, err := c.cfg.OptionSerializer(opts)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/%v", c.url, key)
	resp := c.http.Post(ctx, url, value,
		httputil.WithRawQuery(serializedOptions.AsQuery()),
	)
	if resp.Status == http.StatusOK {
		return nil
	}
	return resp.Error()
}

func (c *Client[V]) Delete(ctx context.Context, key string, opts ...store.Option) error {
	serializedOptions, err := c.cfg.OptionSerializer(opts)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/%v", c.url, key)
	resp := c.http.Delete(ctx, url, nil,
		httputil.WithRawQuery(serializedOptions.AsQuery()),
	)
	if resp.Status == http.StatusNoContent {
		return nil
	}
	return resp.Error()
}

func (c *Client[V]) List(ctx context.Context, opts ...store.Option) (store.ListItems[string, V], error) {
	serializedOptions, err := c.cfg.OptionSerializer(opts)
	if err != nil {
		return nil, err
	}

	var items store.ListItems[string, V]
	resp := c.http.Get(ctx, c.url, httputil.WithRawQuery(serializedOptions.AsQuery())).Parse(&items)
	return items, resp.Error()
}
