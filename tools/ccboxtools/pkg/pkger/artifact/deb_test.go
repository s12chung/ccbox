package artifact

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// debFixtureFileMode is the mode the fixture's non-exec files carry.
const debFixtureFileMode os.FileMode = 0o644

// debFixture stages a fixture tree on disk and packs it into a deb with
// dpkg-deb.
type debFixture struct {
	t       *testing.T
	root    string
	debPath string
}

func (f *debFixture) write(rel string, mode os.FileMode, body string) {
	f.t.Helper()
	path := filepath.Join(f.root, rel)
	require.NoError(f.t, os.MkdirAll(filepath.Dir(path), ExecFileMode))
	require.NoError(f.t, os.WriteFile(path, []byte(body), mode))
}

func (f *debFixture) pack() {
	f.t.Helper()
	// dpkg-deb packs the fixture and extracts it under test — macOS ships none
	if _, err := exec.LookPath("dpkg-deb"); err != nil {
		f.t.Skipf("helper: no dpkg-deb: %v", err)
	}
	f.debPath = filepath.Join(f.t.TempDir(), "fixture.deb")
	//nolint:gosec // fixed argv on the test's own fixture tree
	out, err := exec.CommandContext(context.Background(), "dpkg-deb", "--root-owner-group", "-b", f.root, f.debPath).CombinedOutput()
	require.NoError(f.t, err, string(out))
}

func (f *debFixture) read() []byte {
	f.t.Helper()
	return readFile(f.t, f.debPath)
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	body, err := os.ReadFile(path) //nolint:gosec // test fixture path
	require.NoError(t, err)
	return body
}

func stat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info
}

func entryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func buildDeb(t *testing.T, content []byte) []byte {
	t.Helper()
	f := &debFixture{t: t, root: t.TempDir()}

	const control = "Package: zcode\nVersion: 3.14.4\nArchitecture: all\n" +
		"Maintainer: test <t@t>\nDescription: test fixture\n"
	f.write("DEBIAN/control", debFixtureFileMode, control)
	f.write(filepath.Join("opt", "ZCode", "zcode"), ExecFileMode, string(content))
	f.write(filepath.Join("opt", "ZCode", "resources", "app.ico"), debFixtureFileMode, "ico")
	f.pack()
	return f.read()
}

func TestDeb_Install(t *testing.T) {
	content := []byte("#!/bin/sh\necho zcode\n")
	d := NewDeb("opt/ZCode/zcode")

	dir := t.TempDir()
	require.NoError(t, d.Install(dir, bytes.NewReader(buildDeb(t, content))))

	// the tree extracts intact: the exec with its content and exec bit, and the
	// resource beside it
	binPath := filepath.Join(dir, "opt", "ZCode", "zcode")
	assert.Equal(t, content, readFile(t, binPath))
	assert.NotZero(t, stat(t, binPath).Mode().Perm()&0o100, "want the exec bit")

	stat(t, filepath.Join(dir, "opt", "ZCode", "resources", "app.ico"))

	// the temp deb is gone; only the extracted tree remains
	assert.Equal(t, []string{"opt"}, entryNames(t, dir))

	t.Run("bad deb", func(t *testing.T) {
		dir := t.TempDir()
		err := d.Install(dir, bytes.NewReader([]byte("not a deb")))
		assert.ErrorContains(t, err, "extract deb")
	})
}

func TestNewDeb(t *testing.T) {
	d := NewDeb("opt/ZCode/zcode")
	assert.Equal(t, "opt/ZCode/zcode", d.RelBin())
}
