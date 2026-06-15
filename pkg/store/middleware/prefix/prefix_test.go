package prefix

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ing-bank/golibs/pkg/store"
)

type fakeStore struct {
	calls   []string
	data    map[string]string
	options []store.Option
}

func newFakeStore() *fakeStore {
	return &fakeStore{data: make(map[string]string)}
}

func (f *fakeStore) Create(ctx context.Context, key string, value string, opts ...store.Option) error {
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return err
	}
	f.calls = append(f.calls, "Create:"+key)
	f.data[key] = value
	f.options = opts
	return nil
}
func (f *fakeStore) Read(ctx context.Context, key string, opts ...store.Option) (string, error) {
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return "", err
	}
	f.calls = append(f.calls, "Read:"+key)
	f.options = opts
	v, ok := f.data[key]
	if !ok {
		return "", errors.New("not found")
	}
	return v, nil
}
func (f *fakeStore) Update(ctx context.Context, key string, value string, opts ...store.Option) error {
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return err
	}
	f.calls = append(f.calls, "Update:"+key)
	f.data[key] = value
	f.options = opts
	return nil
}
func (f *fakeStore) Apply(ctx context.Context, key string, value string, opts ...store.Option) error {
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return err
	}
	f.calls = append(f.calls, "Apply:"+key)
	f.data[key] = value
	f.options = opts
	return nil
}
func (f *fakeStore) Delete(ctx context.Context, key string, opts ...store.Option) error {
	if err := store.CheckOptionsExhausted(opts); err != nil {
		return err
	}
	f.calls = append(f.calls, "Delete:"+key)
	delete(f.data, key)
	f.options = opts
	return nil
}
func (f *fakeStore) List(ctx context.Context, opts ...store.Option) (store.ListItems[string, string], error) {
	f.calls = append(f.calls, "List")
	f.options = opts
	var items store.ListItems[string, string]
	for k, v := range f.data {
		items = append(items, store.ListItem[string, string]{Key: k, Value: v})
	}
	return items, nil
}

func TestPrefix_AllMethods(t *testing.T) {
	ctx := context.Background()
	fake := newFakeStore()
	prefix := "pre-"
	db, err := New[string](fake, prefix)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test Create
	err = db.Create(ctx, "foo", "bar")
	if err != nil {
		t.Errorf("Create failed: %v", err)
	}
	if fake.data["pre-foo"] != "bar" {
		t.Errorf("Create did not apply prefix: %+v", fake.data)
	}
	if fake.calls[len(fake.calls)-1] != "Create:pre-foo" {
		t.Errorf("Create did not call with prefixed key: %v", fake.calls)
	}

	// Test Read
	val, err := db.Read(ctx, "foo")
	if err != nil || val != "bar" {
		t.Errorf("Read failed: got %v, want bar, err=%v", val, err)
	}
	if fake.calls[len(fake.calls)-1] != "Read:pre-foo" {
		t.Errorf("Read did not call with prefixed key: %v", fake.calls)
	}

	// Test Update
	err = db.Update(ctx, "foo", "baz")
	if err != nil {
		t.Errorf("Update failed: %v", err)
	}
	if fake.data["pre-foo"] != "baz" {
		t.Errorf("Update did not apply prefix: %+v", fake.data)
	}
	if fake.calls[len(fake.calls)-1] != "Update:pre-foo" {
		t.Errorf("Update did not call with prefixed key: %v", fake.calls)
	}

	// Test Apply
	err = db.Apply(ctx, "foo", "qux")
	if err != nil {
		t.Errorf("Apply failed: %v", err)
	}
	if fake.data["pre-foo"] != "qux" {
		t.Errorf("Apply did not apply prefix: %+v", fake.data)
	}
	if fake.calls[len(fake.calls)-1] != "Apply:pre-foo" {
		t.Errorf("Apply did not call with prefixed key: %v", fake.calls)
	}

	// Test Delete
	err = db.Delete(ctx, "foo")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}
	if _, ok := fake.data["pre-foo"]; ok {
		t.Errorf("Delete did not remove prefixed key: %+v", fake.data)
	}
	if fake.calls[len(fake.calls)-1] != "Delete:pre-foo" {
		t.Errorf("Delete did not call with prefixed key: %v", fake.calls)
	}

	// Test List
	fake.data["pre-a"] = "1"
	fake.data["pre-b"] = "2"
	items, err := db.List(ctx)
	if err != nil {
		t.Errorf("List failed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("List did not return all items: %+v", items)
	}
	if fake.calls[len(fake.calls)-1] != "List" {
		t.Errorf("List did not call List: %v", fake.calls)
	}
}

func TestPrefix_EmptyPrefixError(t *testing.T) {
	fake := newFakeStore()
	_, err := New[string](fake, "")
	if err == nil {
		t.Errorf("expected error for empty prefix, got nil")
	}
}

func TestPrefix_ListWithPrefixOption(t *testing.T) {
	ctx := context.Background()
	fake := newFakeStore()
	prefix := "myprefix-"
	db, _ := New[string](fake, prefix)
	//store := p.(*Prefix[string])
	_, _ = db.List(ctx)
	found := false
	for _, opt := range fake.options { // fake.Options is updated by the mock after the List call
		mytype := reflect.TypeOf(opt).String()
		if mytype == "store.StringOption" {
			found = true // Make sure the list call included the prefix option
			break
		}
	}
	if !found {
		t.Errorf("List did not pass WithPrefix option")
	}
}

func TestPrefix_DoublePrefix(t *testing.T) {
	ctx := context.Background()
	fake := newFakeStore()
	prefix := "p-"
	p, _ := New[string](fake, prefix)
	store := p.(*Prefix[string])
	_ = store.Create(ctx, "foo", "bar")
	// Now call Create again with already-prefixed key
	_ = store.Create(ctx, "p-foo", "baz")
	if fake.data["p-p-foo"] != "baz" {
		t.Errorf("Double prefix not applied as expected: %+v", fake.data)
	}
}

func TestPrefix_OptionsForwarded(t *testing.T) {
	ctx := context.Background()
	fake := newFakeStore()
	prefix := "opt-"
	p, _ := New[string](fake, prefix)
	db := p.(*Prefix[string])
	opt := store.WithPrefix("opt-")
	err := db.Create(ctx, "foo", "bar", opt)
	if err == nil {
		t.Errorf("expected option not supported error")
	}

	opt = store.WithPrefix("opt-")
	_, err = db.List(ctx, opt)
	if err == nil {
		t.Errorf("expected conflicting option error for List with WithPrefix")
	}
}
