// Package yamlutil renders Go values as YAML values for line-oriented templating.
package yamlutil

import (
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// Value renders v as a YAML value for a template line. Unset (nil) renders bare, keeping
// bare key = null = unset — which the yaml library cannot express (it marshals "null"/"[]");
// an empty non-nil slice/map keeps its explicit emptiness (" []"/" {}"); a scalar renders
// inline; anything else renders as a 2-space indented block. Values marshal through yaml,
// so quoting and map-key order match what a yaml parser reads back.
func Value(v any) (string, error) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || (rv.Kind() == reflect.Pointer && rv.IsNil()) {
		return "", nil // unset: the bare key stands
	}
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}

	body, err := yaml.Marshal(rv.Interface())
	if err != nil {
		return "", err
	}
	bodyStr := strings.TrimSuffix(string(body), "\n")

	switch rv.Kind() {
	case reflect.Slice, reflect.Map:
		switch {
		case rv.IsNil():
			return "", nil
		case rv.Len() == 0:
			return " " + bodyStr, nil // yaml renders "[]"/"{}" itself
		default:
			return "\n  " + strings.ReplaceAll(bodyStr, "\n", "\n  "), nil
		}
	default:
		return " " + bodyStr, nil
	}
}
