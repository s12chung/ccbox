package globkit

import (
	"os"
	"path/filepath"
	"testing"

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
