// Package yamlutil contains YAML helpers for reading writing YAML
package yamlutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"reflect"
	"strings"

	"github.com/s12chung/firm"
	"gopkg.in/yaml.v3"
)

// Value renders v as a YAML value for a template line. Unset (nil) renders bare, keeping
// bare key = null = unset — which the yaml library cannot express (it marshals "null"/"[]");
// an empty non-nil slice/map keeps its explicit emptiness (" []"/" {}"); a plain value
// renders inline; a list or map renders as a 2-space indented block. Values marshal
// through yaml, so quoting and map-key order match what a yaml parser reads back.
//
// comments sit beside the rendered lines after " # ": each slice/map entry gets its own,
// so len(comments) must match the entry count or be empty; a plain value — including the
// unset bare key — takes at most one.
func Value(v any, comments []string) (string, error) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || (rv.Kind() == reflect.Pointer && rv.IsNil()) {
		return beside("", comments) // unset: the bare key stands
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
		if len(comments) != 0 && len(comments) != rv.Len() {
			return "", fmt.Errorf("comments: got %d, want %d or none", len(comments), rv.Len())
		}
		switch {
		case rv.IsNil():
			return "", nil
		case rv.Len() == 0:
			return " " + bodyStr, nil // yaml renders "[]"/"{}" itself
		default:
			return "\n  " + strings.ReplaceAll(besideEntries(bodyStr, comments), "\n", "\n  "), nil
		}
	default:
		return beside(" "+bodyStr, comments)
	}
}

// ValidatedDecode decodes one yaml body into T, rejecting unknown fields — a typo'd
// key errors out — and firm-validates the result; setDefaults is called before validations.
func ValidatedDecode[T any](body []byte, setDefaults func(T) T) (T, error) {
	t, err := decode[T](body)
	if err != nil {
		return t, err
	}
	if setDefaults != nil {
		t = setDefaults(t)
	}
	if errMap := firm.ValidateAny(t); errMap != nil {
		return t, errMap
	}
	return t, nil
}

// ReadLayers decodes each path's yaml file as one layer of T, in order — a missing or
// empty file is the zero T, an unset layer. Unknown fields error: a typo'd key errors,
// not silently no-ops.
func ReadLayers[T any](paths []string) ([]T, error) {
	var layers []T
	for _, path := range paths {
		body, err := os.ReadFile(path) // #nosec G304 -- the caller's own layer paths
		if errors.Is(err, fs.ErrNotExist) {
			layers = append(layers, *new(T))
			continue
		}
		if err != nil {
			return layers, err
		}
		layer, err := decode[T](body)
		if err != nil {
			return layers, fmt.Errorf("parse %s: %w", path, err)
		}
		layers = append(layers, layer)
	}
	return layers, nil
}

// ValidatedMerge overlays layers onto a zero T in order — merge folds src into dst, the
// later layer winning — then firm-validates the result: the many-layers counterpart of
// ValidatedDecode.
func ValidatedMerge[T any](layers []T, merge func(dst *T, src T)) (T, error) {
	var acc T
	for _, layer := range layers {
		merge(&acc, layer)
	}
	if errMap := firm.ValidateAny(acc); errMap != nil {
		return acc, errMap
	}
	return acc, nil
}

// decode decodes one yaml body into T, rejecting unknown fields
func decode[T any](body []byte) (T, error) {
	var t T
	dec := yaml.NewDecoder(bytes.NewReader(body))
	dec.KnownFields(true)
	switch err := dec.Decode(&t); {
	case errors.Is(err, io.EOF):
		// when empty (or comment-only) file return zero, will fail on validation
		return t, nil
	case err != nil:
		return t, err
	}
	return t, nil
}

// beside joins at most one comment beside a rendered line
func beside(line string, comments []string) (string, error) {
	switch {
	case len(comments) > 1:
		return "", fmt.Errorf("comments: got %d, want none or one", len(comments))
	case len(comments) == 1 && comments[0] != "":
		return line + " # " + comments[0], nil
	default:
		return line, nil
	}
}

// CommentFirstEntry builds comments for entries, only the first set; nil for an empty list
func CommentFirstEntry(entries []string, comment string) []string {
	if len(entries) == 0 {
		return nil
	}
	comments := make([]string, len(entries))
	comments[0] = comment
	return comments
}

// EntryToComments builds comments for entries looked up in m — missing entries stay bare;
// nil for an empty list
func EntryToComments(entries []string, m map[string]string) []string {
	if len(entries) == 0 {
		return nil
	}
	comments := make([]string, len(entries))
	for i, entry := range entries {
		comments[i] = m[entry]
	}
	return comments
}

// besideEntries joins each comment beside its entry line — the block's column-0 lines,
// one per entry; an empty comment leaves its entry line bare
func besideEntries(block string, comments []string) string {
	lines := strings.Split(block, "\n")
	ci := 0
	for i, line := range lines {
		// ci matches the validated comment count for ordinary blocks, but a key over 128
		// chars (or with newlines) renders as an explicit "? key" + ": value" pair — two
		// column-0 lines for one entry — so the bound still guards comments[ci]
		if ci < len(comments) && !strings.HasPrefix(line, " ") {
			if comments[ci] != "" {
				lines[i] = line + " # " + comments[ci]
			}
			ci++
		}
	}
	return strings.Join(lines, "\n")
}
