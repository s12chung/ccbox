package mise

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderConfig pins the render to the pre-data committed config's bytes:
// equality proves the data-driven render changes no behavior.
func TestRenderConfig(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("testdata", "TestRenderConfig.toml")) // #nosec G304 -- the test's own fixture
	require.NoError(t, err)
	assert.Equal(t, string(golden), string(RenderConfig()))
}

func TestRenderConfig_TemplateErrors(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantErr  string
	}{
		{"unknown action", "{{bogus}}\n", `function "bogus" not defined`},
		{"unrendered section", "[tools]\n", "the tmpl never renders"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := renderConfig([]byte(tt.template))
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
