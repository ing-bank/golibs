package store

import (
	"testing"
)

type TestSerializable string

func (t TestSerializable) Serialize() (string, string) {
	return "test", string(t)
}

func unserializeTest(val string) (Option, error) {
	return TestSerializable(val), nil
}

func TestSerializableOptionBuilder(t *testing.T) {
	var WithTestSerializable, matchTestSerializable = SerializableOptionBuilder[TestSerializable]("test", unserializeTest)
	opts := []Option{WithTestSerializable("abc")}
	serialized, err := SerializeOptions(opts)
	if err != nil {
		t.Fatalf("serialize error: %v", err)
	}
	if v, ok := serialized["test"]; !ok || v != "abc" {
		t.Errorf("expected serialized value 'abc', got '%v'", v)
	}

	unserialized, err := UnserializeOptions(t.Context(), serialized)
	if err != nil {
		t.Fatalf("unserialize error: %v", err)
	}
	val, ok := matchTestSerializable(&unserialized)
	if !ok || val != "abc" {
		t.Errorf("expected unserialized value 'abc', got '%v'", val)
	}
}
