package jmap

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// backRefSpec is the JSON structure of a back-reference argument value per
// RFC 8620 §3.7.
type backRefSpec struct {
	ResultOf string `json:"resultOf"`
	Name     string `json:"name"`
	Path     string `json:"path"`
}

// resolveBackRefs scans args (a JSON object) for keys that start with "#".
// For each such key it resolves the back-reference against prevResults and
// replaces the key (without the leading "#") with the extracted value.
// Keys without "#" are passed through unchanged.
// prevResults maps callID → raw JSON result from a prior MethodCall.
func resolveBackRefs(args json.RawMessage, prevResults map[string]json.RawMessage) (json.RawMessage, error) {
	// Decode args into a map preserving raw JSON values.
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(args, &obj); err != nil {
		return args, nil // not a JSON object — return as-is
	}

	// Check whether any key starts with "#" to avoid unnecessary work.
	hasBackRef := false
	for k := range obj {
		if strings.HasPrefix(k, "#") {
			hasBackRef = true
			break
		}
	}
	if !hasBackRef {
		return args, nil
	}

	// Build a new map, resolving back-refs.
	out := make(map[string]json.RawMessage, len(obj))
	for k, v := range obj {
		if !strings.HasPrefix(k, "#") {
			out[k] = v
			continue
		}

		// Parse the back-reference spec.
		var spec backRefSpec
		if err := json.Unmarshal(v, &spec); err != nil {
			return nil, fmt.Errorf("invalidResultReference: cannot parse back-reference for key %q: %w", k, err)
		}
		if spec.ResultOf == "" || spec.Path == "" {
			return nil, fmt.Errorf("invalidResultReference: back-reference for key %q missing resultOf or path", k)
		}

		// Look up the previous result.
		prevRaw, ok := prevResults[spec.ResultOf]
		if !ok {
			return nil, fmt.Errorf("invalidResultReference: no result for callID %q", spec.ResultOf)
		}

		// Decode the previous result into a generic value.
		var prevVal interface{}
		if err := json.Unmarshal(prevRaw, &prevVal); err != nil {
			return nil, fmt.Errorf("invalidResultReference: cannot decode result for callID %q: %w", spec.ResultOf, err)
		}

		// Walk the path expression.
		extracted, err := walkPath(prevVal, spec.Path)
		if err != nil {
			return nil, fmt.Errorf("invalidResultReference: %w", err)
		}

		// Encode the extracted value.
		encoded, err := json.Marshal(extracted)
		if err != nil {
			return nil, fmt.Errorf("invalidResultReference: cannot encode extracted value: %w", err)
		}

		// Store under the key without the leading "#".
		out[k[1:]] = encoded
	}

	return json.Marshal(out)
}

// walkPath navigates a JSON value (decoded into interface{}) following the
// RFC 8620 §3.7 path format: "/" separated segments where "*" means "collect
// this field from every element of the current array" and numeric segments
// are zero-based array indices.
func walkPath(v interface{}, path string) (interface{}, error) {
	if path == "" || path == "/" {
		return v, nil
	}
	// Strip leading slash.
	p := strings.TrimPrefix(path, "/")
	segments := strings.Split(p, "/")
	return walkSegments(v, segments)
}

func walkSegments(v interface{}, segments []string) (interface{}, error) {
	if len(segments) == 0 {
		return v, nil
	}

	seg := segments[0]
	rest := segments[1:]

	switch seg {
	case "*":
		// Wildcard: v must be an array; collect the result of walking rest
		// for each element.
		arr, ok := v.([]interface{})
		if !ok {
			return nil, fmt.Errorf("path wildcard '*' applied to non-array value")
		}
		results := make([]interface{}, 0, len(arr))
		for _, elem := range arr {
			child, err := walkSegments(elem, rest)
			if err != nil {
				return nil, err
			}
			// Flatten one level: if child is itself a slice (from a nested
			// wildcard), append its elements rather than the slice itself.
			if childSlice, ok := child.([]interface{}); ok && len(rest) == 0 {
				results = append(results, childSlice...)
			} else {
				results = append(results, child)
			}
		}
		return results, nil

	default:
		// Try numeric index first.
		if idx, err := strconv.Atoi(seg); err == nil {
			arr, ok := v.([]interface{})
			if !ok {
				return nil, fmt.Errorf("path index %d applied to non-array value", idx)
			}
			if idx < 0 || idx >= len(arr) {
				return nil, fmt.Errorf("path index %d out of range (len %d)", idx, len(arr))
			}
			return walkSegments(arr[idx], rest)
		}

		// Object key.
		obj, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("path segment %q applied to non-object value", seg)
		}
		child, exists := obj[seg]
		if !exists {
			return nil, fmt.Errorf("path segment %q not found in object", seg)
		}
		return walkSegments(child, rest)
	}
}
