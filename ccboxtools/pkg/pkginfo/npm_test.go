package pkginfo

import (
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNpm_Validate(t *testing.T) {
	tests := []struct {
		name string
		pkg  string
		want []string
	}{
		{"scoped", "@anthropic-ai/claude-code", nil},
		{"plain", "opencode-ai", nil},
		{"empty", "", []string{"Package.Match"}},
		{"uppercase", "MyCLI", []string{"Package.Match"}},
		{"unscoped slash", "foo/bar", []string{"Package.Match"}},
		{"scope without name", "@scope", []string{"Package.Match"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMap := firm.ValidateAny(Npm{Package: tt.pkg})
			if tt.want == nil {
				assert.Nil(t, errMap.ToNil())
				return
			}
			require.NotEmpty(t, errMap)
			for _, want := range tt.want {
				assert.Contains(t, errMap.Error(), want)
			}
		})
	}
}
