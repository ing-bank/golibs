package patch

import (
	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	"testing"
)

func TestMerge(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	original := &TestStruct{Name: "original", Value: 1}
	patch := map[string]any{"name": "patched"}

	merged, err := Merge(original, patch)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	if merged.Name != "patched" {
		t.Errorf("expected name to be 'patched', got '%s'", merged.Name)
	}
	if merged.Value != 1 {
		t.Errorf("expected value to be 1, got %d", merged.Value)
	}
}

func TestJSON(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	original := &TestStruct{Name: "original", Value: 1}
	patch, err := jsonpatch.DecodePatch([]byte(`[{"op": "replace", "path": "/name", "value": "patched"}]`))
	if err != nil {
		t.Fatalf("DecodePatch failed: %v", err)
	}

	merged, err := JSON(original, patch)
	if err != nil {
		t.Fatalf("JSON failed: %v", err)
	}

	if merged.Name != "patched" {
		t.Errorf("expected name to be 'patched', got '%s'", merged.Name)
	}
	if merged.Value != 1 {
		t.Errorf("expected value to be 1, got %d", merged.Value)
	}
}

func TestRaw(t *testing.T) {
	original := []byte(`{"name": "original", "value": 1}`)
	jsonPatch := []byte(`[{"op": "replace", "path": "/name", "value": "patched"}]`)
	mergePatch := []byte(`{"name": "patched"}`)

	// Test JsonPatch
	merged, err := Raw(original, jsonPatch, JsonPatch)
	if err != nil {
		t.Fatalf("Raw failed for JsonPatch: %v", err)
	}

	expected := `{"name":"patched","value":1}`
	if string(merged) != expected {
		t.Errorf("expected '%s', got '%s'", expected, string(merged))
	}

	// Test MergePatch
	merged, err = Raw(original, mergePatch, MergePatch)
	if err != nil {
		t.Fatalf("Raw failed for MergePatch: %v", err)
	}

	if string(merged) != expected {
		t.Errorf("expected '%s', got '%s'", expected, string(merged))
	}

	// Test unsupported patch type
	_, err = Raw(original, mergePatch, "unsupported")
	if err == nil {
		t.Fatalf("expected error for unsupported patch type, got nil")
	}
}
