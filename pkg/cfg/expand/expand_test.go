package expand

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMap(t *testing.T) {
	expand := func(key, value string) (string, string) {
		return "<" + key, value + ">"
	}
	expansions := map[string]Expansion{
		"special": {Key: "resolved-key", Value: func(value string) string { return "resolved:" + value }},
	}

	testcases := []struct {
		name    string
		entries map[string]string
		want    map[string]string
	}{
		{
			name:    "keyed entries expand to their Expansion, the rest ride expand",
			entries: map[string]string{"special": "raw", "plain": "raw"},
			want:    map[string]string{"resolved-key": "resolved:raw", "<plain": "raw>"},
		},
		{
			name:    "no entries expand to nil",
			entries: map[string]string{},
			want:    nil,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Map(tc.entries, expansions, expand))
		})
	}

	t.Run("the raw entries are never touched", func(t *testing.T) {
		entries := map[string]string{"special": "raw", "plain": "raw"}
		Map(entries, expansions, expand)
		assert.Equal(t, map[string]string{"special": "raw", "plain": "raw"}, entries)
	})
}

func TestSlice(t *testing.T) {
	expansions := map[string][]string{"ccbox-defaults": {".idea", ".vscode"}}

	testcases := []struct {
		name    string
		entries []string
		want    []string
	}{
		{
			name:    "keyed entries expand in place, the rest keep as-is",
			entries: []string{"dist", "ccbox-defaults", "example.com"},
			want:    []string{"dist", ".idea", ".vscode", "example.com"},
		},
		{
			name:    "repeats drop, keeping the first occurrence's order",
			entries: []string{"ccbox-defaults", ".idea", "dist", "ccbox-defaults", ".idea"},
			want:    []string{".idea", ".vscode", "dist"},
		},
		{
			name:    "repeated entries drop",
			entries: []string{"dist", "dist"},
			want:    []string{"dist"},
		},
		{
			name:    "no entries expand to nil",
			entries: []string{},
			want:    nil,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Slice(tc.entries, expansions))
		})
	}
}
