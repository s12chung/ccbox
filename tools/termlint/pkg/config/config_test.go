package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/osutil"
)

func TestLoad_Ok(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".termlint.json")
	requireWrite(t, path, `{
		"extensions": [".go", ".md"],
		"exclude_globs": ["dist/**"],
		"terms": [{"word": "wall", "allows": ["egress wall"], "hint": "use egress wall"}]
	}`)

	config, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, []string{".go", ".md"}, config.Extensions)
	assert.Equal(t, []string{"dist/**"}, config.ExcludeGlobs)
	require.Len(t, config.Terms, 1)
	assert.Equal(t, "wall", config.Terms[0].Word)
	assert.Equal(t, []string{"egress wall"}, config.Terms[0].Allows)
	assert.Equal(t, "use egress wall", config.Terms[0].Hint)
}

func TestLoad_Errors(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name:    "UnknownField",
			config:  `{"extensions": [".go"], "terms": [], "typo": true}`,
			wantErr: "unknown field",
		},
		{
			name:    "NoExtensions",
			config:  `{"terms": [{"word": "wall"}]}`,
			wantErr: "extensions: at least one required",
		},
		{
			name:    "EmptyWord",
			config:  `{"extensions": [".go"], "terms": [{"word": "", "hint": "h"}]}`,
			wantErr: "term: word is required",
		},
		{
			name:    "WordlessAllow",
			config:  `{"extensions": [".go"], "terms": [{"word": "wall", "allows": [" / "]}]}`,
			wantErr: `term "wall": allow " / " has no words`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), ".termlint.json")
			requireWrite(t, path, test.config)

			_, err := Load(path)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantErr)
		})
	}
}

func requireWrite(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), osutil.File)) // #nosec G306 -- the test's own fixture
}
