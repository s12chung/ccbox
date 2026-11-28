// Package jsonutil parses strict JSON bodies: unknown fields and
// invalid values rejected.
package jsonutil

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/s12chung/firm"
)

// Decode parses a JSON body into T — a parse failure names the body.
func Decode[T any](body string) (T, error) {
	var v T
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, fmt.Errorf("parse %s: %w", body, err)
	}
	if errMap := firm.ValidateAny(v); errMap != nil {
		return v, errMap
	}
	return v, nil
}
