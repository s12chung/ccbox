package fsutil

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

func srcFS() fstest.MapFS {
	return fstest.MapFS{
		"AGENTS.md":         {Data: []byte("new agents")},
		"settings.json":     {Data: []byte("new settings")},
		"hooks/tripwire.sh": {Data: []byte("new hook")}, // nested dir exercised
	}
}

// writeFS lays fsys's tree onto dest
func writeFS(t *testing.T, fsys fs.FS, dest string) {
	t.Helper()
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dest, p), ioutil.Dir)
		}
		body, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dest, p), body, ioutil.File)
	})
	require.NoError(t, err)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(body), ioutil.File))
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(path, ioutil.Dir))
}

func TestMatches(t *testing.T) {
	tests := []struct {
		name  string
		write func(t *testing.T, dest string)
		want  bool
	}{
		{name: "identical tree matches", write: func(t *testing.T, dest string) {
			writeFS(t, srcFS(), dest)
		}, want: true},
		{name: "modified file mismatches", write: func(t *testing.T, dest string) {
			writeFS(t, srcFS(), dest)
			writeFile(t, filepath.Join(dest, "settings.json"), "edited")
		}, want: false},
		{name: "added file mismatches", write: func(t *testing.T, dest string) {
			writeFS(t, srcFS(), dest)
			writeFile(t, filepath.Join(dest, "hooks/extra.sh"), "added")
		}, want: false},
		{name: "missing file mismatches", write: func(t *testing.T, dest string) {
			writeFS(t, srcFS(), dest)
			require.NoError(t, os.Remove(filepath.Join(dest, "AGENTS.md")))
		}, want: false},
		{name: "added empty dir mismatches", write: func(t *testing.T, dest string) {
			writeFS(t, srcFS(), dest)
			mkdirAll(t, filepath.Join(dest, "extra"))
		}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := t.TempDir()
			tt.write(t, dest)

			matches, err := Matches(srcFS(), dest)
			require.NoError(t, err)
			assert.Equal(t, tt.want, matches)
		})
	}
}
