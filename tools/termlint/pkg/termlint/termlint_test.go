package termlint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/osutil"
)

func TestRun_Clean(t *testing.T) {
	root := writeTree(t, map[string]string{
		".termlint.json": `{
			"extensions": [".go", ".md"],
			"exclude_globs": [".termlint.json", "tools/**"],
			"terms": [{"word": "wall", "allows": ["egress wall"], "hint": "use egress wall"}]
		}`,
		"main.go":    "package main\nvar proxy = egressWall\n",
		"notes.md":   "behind the egress wall, EgressWall, egress-wall\n",
		"types.go":   "package main\ntype SwallowErr struct{}\nvar wallet = 1\n",
		"data.txt":   "wall\n",
		"tools/x.md": "wall\n",
	})

	err := Run(root)
	require.NoError(t, err)
}

func TestRun_Violations(t *testing.T) {
	root := writeTree(t, map[string]string{
		".termlint.json": `{
			"extensions": [".go", ".md"],
			"exclude_globs": [".termlint.json"],
			"terms": [
				{"word": "wall", "hint": "use egress wall"},
				{"word": "ride", "hint": "name the relationship"}
			]
		}`,
		"main.go":  "package main\nvar w = NewWall()\n",
		"notes.md": "line one\nride along\n",
		"other.go": "package main\n// egress wall, then a bare wall\nvar w = NewWalls()\n",
	})

	err := Run(root)
	require.Error(t, err)
	assert.Equal(t, `main.go:2:var w = NewWall()
other.go:2:// egress wall, then a bare wall
other.go:3:var w = NewWalls()

use egress wall
notes.md:2:ride along

name the relationship`, err.Error())
}

func TestRun_SkipsGitAndExcluded(t *testing.T) {
	root := writeTree(t, map[string]string{
		".termlint.json": `{
			"extensions": [".go", ".md"],
			"exclude_globs": [".termlint.json", "vendor/**"],
			"terms": [{"word": "wall", "hint": "use egress wall"}]
		}`,
		".git/config":     "wall\n",
		"vendor/lib/x.md": "wall\n",
	})

	err := Run(root)
	require.NoError(t, err)
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for path, content := range files {
		path := filepath.Join(root, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), osutil.Dir))      // #nosec G301 -- the test's own fixture dir
		require.NoError(t, os.WriteFile(path, []byte(content), osutil.File)) // #nosec G306 -- the test's own fixture
	}
	return root
}
