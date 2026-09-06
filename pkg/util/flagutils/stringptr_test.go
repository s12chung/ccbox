package flagutils

import (
	"flag"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parser is the Parse shape shared by flag.FlagSet and pflag.FlagSet.
type parser interface {
	Parse([]string) error
}

// TestStringPtr runs the flag behavior through both flag libraries: the stdlib
// flag and pflag, which cobra binds.
func TestStringPtr(t *testing.T) {
	flavors := map[string]func(p **string) parser{
		"stdlib": func(p **string) parser {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			fs.Var(StringPtr(p), "cli", "")
			return fs
		},
		"pflag": func(p **string) parser {
			fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
			fs.Var(StringPtr(p), "cli", "")
			return fs
		},
	}

	tests := []struct {
		name string
		args []string
		want *string
	}{
		{"unset stays nil", nil, nil},
		{"set writes through", []string{"--cli=codex"}, new("codex")},
		{"explicit empty is set, not unset", []string{"--cli="}, new("")},
	}

	for flavor, bind := range flavors {
		for _, tt := range tests {
			t.Run(flavor+"/"+tt.name, func(t *testing.T) {
				var v *string
				require.NoError(t, bind(&v).Parse(tt.args))
				assert.Equal(t, tt.want, v)
			})
		}
	}
}

func TestStringPtrValueString(t *testing.T) {
	var v *string
	value := StringPtr(&v)
	assert.Empty(t, value.String()) // nil target: the unset rendering

	require.NoError(t, value.Set("codex"))
	assert.Equal(t, "codex", value.String())

	var zero StringPtrValue
	assert.Empty(t, zero.String()) // flag's zero-value reflection must not panic
}
