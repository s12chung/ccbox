package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/toinnermodel-gen/pkg/source"
)

func TestRun(t *testing.T) {
	fixtures := []string{
		filepath.Join("pkg", "render", "testdata", "single"),
		filepath.Join("pkg", "render", "testdata", "multi"),
	}

	t.Run("a missing file is written", func(t *testing.T) {
		for _, fixture := range fixtures {
			dir := t.TempDir()
			copyViews(t, fixture, dir)
			require.NoError(t, run(dir))
			assertGenerated(t, fixture, dir)
		}
	})

	t.Run("a stale file is rewritten", func(t *testing.T) {
		for _, fixture := range fixtures {
			dir := t.TempDir()
			writeStale(t, fixture, dir)
			copyViews(t, fixture, dir)
			require.NoError(t, run(dir))
			assertGenerated(t, fixture, dir)
		}
	})

	t.Run("a missing dir refuses", func(t *testing.T) {
		require.Error(t, run(filepath.Join(t.TempDir(), "missing")))
	})

	t.Run("a marked struct with no innerModel refuses", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "lone.go", `package lone

//toinnermodel:gen
type lone struct {
	Name string
}
`)
		err := run(dir)
		require.Error(t, err)
		assert.ErrorContains(t, err, "no innerModel struct Lone in the package")
	})
}

// copyViews copies src's hand-written sources into dst, leaving the generated
// files behind: run must write them
func copyViews(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), source.GeneratedPrefix) {
			continue
		}
		body, err := os.ReadFile(filepath.Join(src, entry.Name())) // #nosec G304 -- the test's own fixture dir
		require.NoError(t, err)
		writeFile(t, dst, entry.Name(), string(body))
	}
}

// writeStale writes each fixture's generated file stale, so run must
// rewrite it
func writeStale(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), source.GeneratedPrefix) {
			continue
		}
		writeFile(t, dst, entry.Name(), "// stale\n")
	}
}

// assertGenerated requires dir's generated files to match the fixture's golden
func assertGenerated(t *testing.T, fixture, dir string) {
	t.Helper()
	entries, err := os.ReadDir(fixture)
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), source.GeneratedPrefix) {
			continue
		}
		golden, err := os.ReadFile(filepath.Join(fixture, entry.Name())) // #nosec G304 -- the test's own golden file
		require.NoError(t, err)
		written, err := os.ReadFile(filepath.Join(dir, entry.Name())) // #nosec G304 -- the test's own output path
		require.NoError(t, err)
		assert.Equal(t, string(golden), string(written), entry.Name())
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), osutil.File)) // #nosec G703 -- the test's own output path
}
