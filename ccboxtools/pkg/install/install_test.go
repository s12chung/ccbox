package install

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkger"
)

// fakePkger serves a canned latest version; Install simulates npm's bin layout.
type fakePkger struct {
	name      string
	latest    string
	latestErr error

	installed []string
}

func (f *fakePkger) Name() string            { return f.name }
func (f *fakePkger) Latest() (string, error) { return f.latest, f.latestErr }
func (f *fakePkger) RelBin() string          { return "bin/" + f.name }
func (f *fakePkger) Install(dir, version string) error {
	f.installed = append(f.installed, version)
	if err := os.MkdirAll(filepath.Join(dir, "bin"), dirMode); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "bin", f.name), nil, dirMode)
}

func readLink(t *testing.T, path string) string {
	t.Helper()
	target, err := os.Readlink(path)
	require.NoError(t, err)
	return target
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

func run(p *fakePkger, root string) error {
	return Run(pkger.PkgDir{Pkger: p, Root: root})
}

func TestRun_Installs(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := t.TempDir()

	require.NoError(t, run(p, root))

	// version dir in place, current flipped, bin exposed
	require.DirExists(t, filepath.Join(root, "claude", "1.2.3"))
	assert.Equal(t, "1.2.3", readLink(t, filepath.Join(root, "claude", "current")))
	assert.Equal(t,
		filepath.Join("..", "claude", "current", "bin", "claude"),
		readLink(t, filepath.Join(root, "bin", "claude")))
	assert.Equal(t, []string{"1.2.3", "current"}, entryNames(t, filepath.Join(root, "claude")))
}

func TestRun_UpgradesAndPrunes(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := t.TempDir()
	require.NoError(t, run(p, root))

	p.latest = "2.0.0"
	require.NoError(t, run(p, root))

	assert.Equal(t, []string{"1.2.3", "2.0.0"}, p.installed)
	assert.Equal(t, "2.0.0", readLink(t, filepath.Join(root, "claude", "current")))
	assert.Equal(t, []string{"2.0.0", "current"}, entryNames(t, filepath.Join(root, "claude")))
}

func TestRun_UpToDate(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := t.TempDir()
	require.NoError(t, run(p, root))
	require.NoError(t, run(p, root))

	assert.Equal(t, []string{"1.2.3"}, p.installed) // no re-install on the second run
}

func TestRun_ChannelFails_FallsBackToStale(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := t.TempDir()
	require.NoError(t, run(p, root))

	p.latest, p.latestErr = "", errors.New("channel down")
	require.NoError(t, run(p, root)) // the installed 1.2.3 stands in

	assert.Equal(t, []string{"1.2.3"}, p.installed)
	assert.Equal(t, "1.2.3", readLink(t, filepath.Join(root, "claude", "current")))
}

func TestRun_ChannelFails_NothingInstalled(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "", latestErr: errors.New("channel down")}

	err := Run(pkger.PkgDir{Pkger: p, Root: t.TempDir()})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel down")
}
