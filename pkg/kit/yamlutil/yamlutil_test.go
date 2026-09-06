package yamlutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValue(t *testing.T) {
	foo := "foo"
	tests := []struct {
		name string
		v    any
		want string
	}{
		{"nil interface", nil, ""},
		{"nil pointer", (*string)(nil), ""},
		{"nil slice", []string(nil), ""},
		{"nil map", map[string]string(nil), ""},
		{"pointer value", &foo, " foo"},
		{"string", "claude", " claude"},
		{"bool", true, " true"},
		{"quoted string", "yes", " \"yes\""}, // yaml quotes bool-like strings

		{"empty slice", []string{}, " []"},
		{"empty map", map[string]string{}, " {}"},
		{"slice", []string{"a", "b"}, "\n  - a\n  - b"},
		{"map sorts keys", map[string]string{"b": "1", "a": "2"}, "\n  a: \"2\"\n  b: \"1\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Value(tt.v)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
