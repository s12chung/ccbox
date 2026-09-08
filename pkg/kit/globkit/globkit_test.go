package globkit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gobwas/glob"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

func TestDoubleStarRooted(t *testing.T) {
	tests := []struct {
		name     string
		glob     string
		match    []string
		notMatch []string
	}{
		{
			"a glob without a leading **/ compiles as-is, * spanning a segment only",
			"certs/*.pem",
			[]string{"certs/server.pem"},
			[]string{"server.pem", "certs/deep/server.pem"},
		},
		{
			"a leading **/ spans directories and matches the root via the stripped twin",
			"**/*.pem",
			[]string{"certs/server.pem", "deep/certs/server.pem", "server.pem"},
			[]string{"certs/server.txt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := DoubleStarRooted(tt.glob)
			for _, s := range tt.match {
				assert.Truef(t, g.Match(s), "%q should match %q", tt.glob, s)
			}
			for _, s := range tt.notMatch {
				assert.Falsef(t, g.Match(s), "%q should not match %q", tt.glob, s)
			}
		})
	}

	t.Run("the twin exists only when there is a leading **/", func(t *testing.T) {
		_, rooted := DoubleStarRooted("**/*.pem").(doubleStarRooted)
		assert.True(t, rooted)
		_, rooted = DoubleStarRooted("certs/*.pem").(doubleStarRooted)
		assert.False(t, rooted)
	})
}

func TestWalkMatches(t *testing.T) {
	write := func(t *testing.T, paths ...string) string {
		t.Helper()
		dir := t.TempDir()
		for _, p := range paths {
			abs := filepath.Join(dir, p)
			require.NoError(t, os.MkdirAll(filepath.Dir(abs), ioutil.Dir))
			require.NoError(t, os.WriteFile(abs, nil, ioutil.File))
		}
		return dir
	}

	t.Run("matches files and dirs by rel path, in walk order, root excluded", func(t *testing.T) {
		root := write(t, ".env", "certs/server.pem")
		matches := WalkMatches(root, DoubleStarRooted(".env"), DoubleStarRooted("certs"))
		assert.Equal(t, []string{".env", "certs"}, matches)
	})

	t.Run("a matching dir is reported once, its contents matching on their own", func(t *testing.T) {
		root := write(t, "certs/server.pem")
		assert.Equal(t, []string{"certs"}, WalkMatches(root, DoubleStarRooted("certs")))
		assert.Equal(t, []string{"certs/server.pem"}, WalkMatches(root, DoubleStarRooted("**/*.pem")))
	})

	t.Run("the read-only defaults together match every sensitive path, in walk order", func(t *testing.T) {
		globs := []string{
			".ccbox.yaml", ".ccbox.local.yaml",
			".env", ".env.*", ".envrc",
			"secrets",
			"**/*.pem", "**/*.key",
		}
		root := write(t,
			// one root-level representative per glob, including **/'s stripped-twin root match
			".ccbox.yaml", ".ccbox.local.yaml",
			".env", ".env.local", ".envrc",
			"server.pem",
			// deeper matches: **/ spanning dirs, and the "secrets" dir itself
			"certs/ca.pem", "certs/server.key", "certs/server.pem",
			"secrets/api.key", "secrets/server.pem",
			"deep/db.key", "deep/nested/old.key", "deep/nested/old.pem",
			// non-matching fillers, keeping more than two files per directory
			"notes.txt",
			"certs/readme.txt",
			"secrets/notes.txt",
			"deep/config.yaml", "deep/notes.txt",
			"deep/nested/notes.txt",
			// exact globs stay rooted: nested .env files and a nested secrets/ dir don't match
			"deep/.env", "deep/.env.local",
			"deep/secrets/notes.txt", "deep/secrets/readme.txt", "deep/secrets/config.yaml",
		)
		matchers := make([]glob.Glob, len(globs))
		for i, g := range globs {
			matchers[i] = DoubleStarRooted(g)
		}

		assert.Equal(t, []string{
			".ccbox.local.yaml", ".ccbox.yaml",
			".env", ".env.local", ".envrc",
			"certs/ca.pem", "certs/server.key", "certs/server.pem",
			"deep/db.key",
			"deep/nested/old.key", "deep/nested/old.pem",
			"secrets", "secrets/api.key", "secrets/server.pem",
			"server.pem",
		}, WalkMatches(root, matchers...))
	})

	t.Run("nothing matches yet — nil", func(t *testing.T) {
		root := write(t, ".env")
		assert.Nil(t, WalkMatches(root, DoubleStarRooted("target")))
		assert.Nil(t, WalkMatches(root))
	})

	t.Run("an absent root matches nothing", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing")
		assert.Nil(t, WalkMatches(missing, DoubleStarRooted("**/*")))
	})
}
