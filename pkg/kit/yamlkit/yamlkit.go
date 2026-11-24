// Package yamlkit contains YAML helpers for reading and writing YAML.
package yamlkit

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/s12chung/firm"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/slicex"
)

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
// empty file is the zero T, an unset layer.
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

// MergeLayers overlays layers onto a zero T in order — merge folds src into dst, the
// later layer winning.
func MergeLayers[T any](layers []T, merge func(dst *T, src T)) T {
	var acc T
	for _, layer := range layers {
		merge(&acc, layer)
	}
	return acc
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

// Value renders v as a YAML value for a template line, comments beside the
// rendered lines — nil stays a bare key, an empty slice or map stays " []"/" {}".
func Value(v any, comments []string) (string, error) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || (rv.Kind() == reflect.Pointer && rv.IsNil()) {
		return beside("", comments) // unset: the bare key stands
	}
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}

	// through yaml, so quoting matches what a yaml parser reads back
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
			return "", nil // a nil slice or map is unset too — yaml would render "[]"
		case rv.Len() == 0:
			return " " + bodyStr, nil // explicitly empty, not unset — yaml renders "[]"/"{}" itself
		default:
			return "\n  " + strings.ReplaceAll(besideEntries(bodyStr, comments), "\n", "\n  "), nil
		}
	default:
		return beside(" "+bodyStr, comments)
	}
}

// Flow renders values as a multi-line YAML flow sequence for a template line.
func Flow(values []string, groups map[string]int) string {
	switch {
	case values == nil:
		return "" // unset: the bare key stands
	case len(values) == 0:
		return " []" // explicit emptiness
	}
	lines := slicex.Map(flowRows(values, groups), func(row []string) string {
		entries := slicex.Map(row, func(v string) string {
			// through yaml, so quoting matches what a yaml parser reads back
			return strings.TrimSpace(string(must.Get(yaml.Marshal(v))))
		})
		return "  " + strings.Join(entries, ", ") + ","
	})
	return " [\n" + strings.Join(lines, "\n") + "\n]"
}

func flowRows(values []string, groups map[string]int) [][]string {
	rows := [][]string{}
	var rest []string
	for _, value := range values {
		index, ok := groups[value]
		if !ok {
			rest = append(rest, value)
			continue
		}
		for len(rows) <= index {
			rows = append(rows, nil)
		}
		rows[index] = append(rows[index], value) // same groups index shares a row, in index order
	}
	rows = slicex.Filter(rows, func(row []string) bool { return row != nil }) // an index no value hits renders nothing
	for _, value := range rest {
		rows = append(rows, []string{value}) // outside groups: a row each, after
	}
	return rows
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

// Indent prefixes each non-empty line of s with n spaces — nesting a Value block under
// an already-indented key.
func Indent(n int, s string) string {
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = pad + line
		}
	}
	return strings.Join(lines, "\n")
}

var (
	proseLine = regexp.MustCompile(`(?m)^[ \t]*##.*\n?`)
	proseTail = regexp.MustCompile(`[ \t]+##.*`) // whitespace-led: a "##" glued to a token is content
	blankRun  = regexp.MustCompile(`\n{3,}`)
)

// StripProse drops the prose — a "##"-led line whole, any other line just its "##" tail.
// Blank runs collapse to one blank line, ends trimmed.
func StripProse(body string) string {
	body = proseLine.ReplaceAllString(body, "")
	body = proseTail.ReplaceAllString(body, "")
	body = blankRun.ReplaceAllString(body, "\n\n")
	if body = strings.Trim(body, "\n"); body == "" {
		return ""
	}
	return body + "\n"
}

// besideEntries joins each comment beside its entry line — the block's column-0 lines,
// one per entry; an empty comment leaves its entry line bare
func besideEntries(block string, comments []string) string {
	lines := strings.Split(block, "\n")
	ci := 0 // ci matches `comments` length as long keys or with \n renders as an explicit k/v pair
	for i, line := range lines {
		if ci < len(comments) && !strings.HasPrefix(line, " ") {
			if comments[ci] != "" {
				lines[i] = line + " # " + comments[ci]
			}
			ci++
		}
	}
	return strings.Join(lines, "\n")
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

// EntryToComments builds Value's comments for entries looked up in m — missing entries stay
// bare; nil for no entries. A map's entries are its keys, in the order yaml renders them.
func EntryToComments[E []string | map[string]string](entries E, m map[string]string) []string {
	keys, isSlice := any(entries).([]string)
	if !isSlice {
		// a round trip, so the key order is yaml's own
		var doc yaml.Node
		must.Do(yaml.Unmarshal(must.Get(yaml.Marshal(entries)), &doc))
		for i, key := range doc.Content[0].Content {
			if i%2 == 0 { // a mapping's content alternates key, value
				keys = append(keys, key.Value)
			}
		}
	}
	return slicex.Map(keys, func(key string) string { return m[key] })
}
