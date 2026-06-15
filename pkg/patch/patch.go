// Package patch provides utilities for applying JSON Patch and Merge Patch operations to Go structs.
//
// JSON Patch (RFC 6902) is a format for expressing a sequence of operations to apply to a JSON document.
// It allows for operations such as add, remove, replace, move, copy, and test. More information can be found at:
// https://tools.ietf.org/html/rfc6902
//
// Merge Patch (RFC 7386) is a simpler format for expressing changes to a JSON document. It is primarily used for
// partial updates and is less expressive than JSON Patch. More information can be found at:
// https://tools.ietf.org/html/rfc7386
//
// This package provides functions to apply these patches to Go structs, allowing for flexible and efficient
// modifications to structured data.

package patch

import (
	"encoding/json"
	"fmt"
	jsonpatch "gopkg.in/evanphx/json-patch.v4"
)

// Type represents the type of patch operation.
type Type string

const (
	JsonPatch  Type = "application/json-patch+json"
	MergePatch Type = "application/merge-patch+json"
)

var (
	ErrUnsupportedPatchType = fmt.Errorf("unsupported patch type")
)

// Merge applies a merge patch to the original object.
// It returns the merged object or an error if the operation fails.
//
// Parameters:
//
//	original - The original object to be patched. Must be compatible with JSON marshal.
//	patch - A map representing the patch to be applied. Must be compatible with JSON marshal.
//
// Returns:
//
//	*T - The merged object.
//	error - An error if the merge operation fails.
func Merge[T any](original *T, patch map[string]any) (*T, error) {
	rawOriginal, err := json.Marshal(original)
	if err != nil {
		return nil, err
	}

	rawPatch, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}

	rawMerged, err := Raw(rawOriginal, rawPatch, MergePatch)
	if err != nil {
		return nil, err
	}
	merged := new(T)
	return merged, json.Unmarshal(rawMerged, merged)
}

// JSON applies a JSON Patch to the original object.
// It returns the patched object or an error if the operation fails.
//
// Parameters:
//
//	original - The original object to be patched. Must be compatible with JSON marshal.
//	patch - A jsonpatch.Patch representing the patch to be applied.
//
// Returns:
//
//	*T - The patched object.
//	error - An error if the patch operation fails.
func JSON[T any](original *T, patch jsonpatch.Patch) (*T, error) {
	rawOriginal, err := json.Marshal(original)
	if err != nil {
		return nil, err
	}

	rawMerged, err := patch.Apply(rawOriginal)
	if err != nil {
		return nil, err
	}

	merged := new(T)
	return merged, json.Unmarshal(rawMerged, merged)
}

// Raw applies a patch to the original byte slice based on the patch type.
// It returns the patched object, as bytes, or an error if the operation fails.
//
// Parameters:
//
//	original - The original object, as bytes, to be patched.
//	patch - The byte slice representing the patch to be applied.
//	patchType - The type of patch operation (JsonPatch or MergePatch).
//
// Returns:
//
//	[]byte - The patched object, as bytes.
//	error - An error if the patch operation fails.
func Raw(original, patch []byte, patchType Type) ([]byte, error) {
	if patchType == JsonPatch {
		decodedPatch, err := jsonpatch.DecodePatch(patch)
		if err != nil {
			return nil, err
		}

		return decodedPatch.Apply(original)
	}

	if patchType == MergePatch {
		return jsonpatch.MergePatch(original, patch)
	}

	return nil, ErrUnsupportedPatchType
}
