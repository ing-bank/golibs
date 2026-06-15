// Package server provides a generic HTTP server for CRUD operations on a store.Store.
// It exposes endpoints for create, read, update, delete, list, and apply operations.
// The server is generic over any type V that implements the Nameable interface.
//
// Example usage:
//
//	type MyType struct { Name string }
//	func (m MyType) GetName() string { return m.Name }
//
//	s := server.New(store, parser) // parser may be nil
//	r := gin.Default()
//	s.Register(r.Group("/v1"))
package server

import (
	goerrors "errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ing-bank/golibs/pkg/errors"
	"github.com/ing-bank/golibs/pkg/opt"
	"github.com/ing-bank/golibs/pkg/store"
	httpstore "github.com/ing-bank/golibs/pkg/store/backends/http"
	"github.com/ing-bank/golibs/pkg/store/backends/labels"
)

type Config struct {
	OptionParser       store.OptionsParser `json:"-"`
	PluralResourceName string              `json:"pluralResourceName"` // E.g. Namespaces
	ResourceVersion    string              `json:"resourceVersion"`    // E.g. v1

	// UseLabels sets the ResourceVersion and PluralResourceName as labels in the store backend.
	// The backend MUST support labels, otherwise setting this option will cause errors.
	UseLabels bool `json:"useLabels"`
}

func (c *Config) ApplyDefaults() {
	if c.OptionParser == nil {
		c.OptionParser = store.UnserializeOptions
	}
}

func (c *Config) Validate() error {
	c.ApplyDefaults()
	if c.OptionParser == nil {
		return fmt.Errorf("option parser is required")
	}
	if c.UseLabels && (c.ResourceVersion == "" || c.PluralResourceName == "") {
		return fmt.Errorf("resource version and plural resource name is required when using labels")
	}
	return nil
}

// Server provides HTTP handlers for a generic store.Store.
// It supports create, read, update, delete, list, and apply operations.
type Server[V httpstore.ValidatableNameable] struct {
	store store.Store[string, V]
	cfg   *Config
}

// New creates a new Server with default options parser
func New[V httpstore.ValidatableNameable](s store.Store[string, V], optCfg ...*Config) (*Server[V], error) {
	cfg := opt.Opt(&Config{}, optCfg)
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Server[V]{
		store: s,
		cfg:   cfg,
	}, nil
}

// Register registers all HTTP handlers on the given Gin router group.
// Endpoints:
//
//	POST   /        - create
//	GET    /:name   - read
//	PUT    /:name   - update
//	DELETE /:name   - delete
//	GET    /        - list
//	POST   /:name   - apply
func (p Server[V]) Register(rg gin.IRouter) {
	prefix := strings.Join([]string{p.cfg.ResourceVersion, p.cfg.PluralResourceName}, "/")

	rg.POST(prefix, p.create)
	rg.GET(prefix+"/:name", p.read)
	rg.PUT(prefix+"/:name", p.update)
	rg.DELETE(prefix+"/:name", p.delete)
	rg.GET(prefix, p.list)
	rg.POST(prefix+"/:name", p.apply)
}

func (p Server[V]) GetLabels() map[string]string {
	return map[string]string{
		"resourceVersion":    p.cfg.ResourceVersion,
		"pluralResourceName": p.cfg.PluralResourceName,
	}
}

func (p Server[V]) SetLabelsIfNeeded(opts *[]store.Option) error {
	if !p.cfg.UseLabels {
		return nil
	}

	return labels.EnrichWithLabelsOption(p.GetLabels(), opts)
}

// GetOptions parses query parameters into store options using the configured parser.
func (p Server[V]) GetOptions(c *gin.Context) ([]store.Option, error) {
	if p.cfg.OptionParser == nil {
		return nil, goerrors.New("empty option parser")
	}

	query := c.Request.URL.Query()
	flat := make(map[string]string)

	for key, values := range query {
		if len(values) > 0 {
			flat[key] = values[0] // take the first value
		} else {
			flat[key] = ""
		}
	}

	return p.cfg.OptionParser(c.Request.Context(), flat)
}

// create handles POST / and creates a new item in the store.
// Returns 201 Created on success, 409 Conflict if item exists.
func (p Server[V]) create(c *gin.Context) {
	opts, err := p.GetOptions(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := p.SetLabelsIfNeeded(&opts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}

	// Parse body
	var body V
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := body.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = p.store.Create(c.Request.Context(), body.GetName(), body, opts...)
	if err != nil {
		if goerrors.Is(err, errors.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else if goerrors.Is(err, store.ErrUnsupportedOption) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	}
	c.Status(http.StatusCreated)
}

// read handles GET /:name and returns the item with the given name.
// Returns 200 OK on success, 404 Not Found if item does not exist.
func (p Server[V]) read(c *gin.Context) {
	opts, err := p.GetOptions(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	name := c.Param("name")
	val, err := p.store.Read(c.Request.Context(), name, opts...)
	if err != nil {
		if goerrors.Is(err, errors.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else if goerrors.Is(err, store.ErrUnsupportedOption) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, val)
}

// update handles PUT /:name and updates the item with the given name.
// Returns 200 OK on success, 404 Not Found if item does not exist.
func (p Server[V]) update(c *gin.Context) {
	opts, err := p.GetOptions(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := p.SetLabelsIfNeeded(&opts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}

	name := c.Param("name")
	var body V
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := body.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = p.store.Update(c.Request.Context(), name, body, opts...)
	if err != nil {
		if goerrors.Is(err, errors.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else if goerrors.Is(err, store.ErrUnsupportedOption) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.Status(http.StatusOK)
}

// delete handles DELETE /:name and deletes the item with the given name.
// Returns 204 No Content on success, 404 Not Found if item does not exist.
func (p Server[V]) delete(c *gin.Context) {
	opts, err := p.GetOptions(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	name := c.Param("name")
	err = p.store.Delete(c.Request.Context(), name, opts...)
	if err != nil {
		if goerrors.Is(err, errors.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else if goerrors.Is(err, store.ErrUnsupportedOption) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// list handles GET / and returns all items in the store.
// Returns 200 OK with a list of items.
func (p Server[V]) list(c *gin.Context) {
	opts, err := p.GetOptions(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if p.cfg.UseLabels {
		if err := labels.EnrichWithLabelSelectorOption(p.GetLabels(), &opts); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
	}

	items, err := p.store.List(c.Request.Context(), opts...)
	if err != nil {
		if goerrors.Is(err, store.ErrUnsupportedOption) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

// apply handles POST /:name and applies the given item to the store.
// The name in the body must match the path name.
// Returns 200 OK on success, 400 Bad Request if names do not match.
func (p Server[V]) apply(c *gin.Context) {
	opts, err := p.GetOptions(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := p.SetLabelsIfNeeded(&opts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}

	name := c.Param("name")
	var body V
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := body.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if body.GetName() != name {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body name does not match path name"})
		return
	}
	err = p.store.Apply(c.Request.Context(), name, body, opts...)
	if err != nil {
		if goerrors.Is(err, store.ErrUnsupportedOption) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusOK)
}
