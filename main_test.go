package main

import (
	"bufio"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/dockerfile"
	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// TestBuildContextHasCopySources guards the context/Dockerfile coupling: every
// path the Dockerfile COPYs from the context must be in it
func TestBuildContextHasCopySources(t *testing.T) {
	src := composedContext(t)

	f, err := src.Open("Dockerfile")
	require.NoError(t, err, "Dockerfile not in the build context")
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 0 || fields[0] != "COPY" {
			continue
		}

		fromStage := false
		var nonFlag []string
		for _, a := range fields[1:] {
			switch {
			case strings.HasPrefix(a, "--from="):
				fromStage = true
			case !strings.HasPrefix(a, "--"):
				nonFlag = append(nonFlag, a)
			}
		}
		// --from copies from a build stage, not the context; the last arg is the dest.
		if fromStage || len(nonFlag) < 2 {
			continue
		}
		for _, path := range nonFlag[:len(nonFlag)-1] {
			path = strings.TrimSuffix(path, "/") // dir COPYs carry a trailing slash
			_, err := fs.Stat(src, path)
			require.NoErrorf(t, err, "Dockerfile COPYs %q but it isn't in the build context", path)
		}
	}
	require.NoError(t, sc.Err())
}

// TestBuildContext_Golden pins the composed context's file set — the files'
// bytes pin in their owners' tests
func TestBuildContext_Golden(t *testing.T) {
	src := composedContext(t)
	assert.Equal(t, goldenPaths, strings.Join(contextPaths(t, src), "\n")+"\n")
}

// composedContext mirrors `ccbox build`'s build context assembly
func composedContext(t *testing.T) fs.FS {
	t.Helper()
	testutil.Home(t) // keeps the test off the host's user level
	require.NoError(t, mise.SeedConfig(mise.UserConfigPath(), runtime.AllMiseTools()))
	require.NoError(t, os.WriteFile(mise.LockPath(mise.UserConfigPath()), []byte("lock"), osutil.File))
	src, err := mise.BuildFS(embedBuildContext, t.TempDir(), runtime.AllMiseTools())
	require.NoError(t, err)
	src, err = dockerfile.PatchFS(src)
	require.NoError(t, err)
	return src
}

// contextPaths lists the composed context's files in the walk's lexical order
func contextPaths(t *testing.T, src fs.FS) []string {
	t.Helper()
	var paths []string
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, p)
		}
		return nil
	})
	require.NoError(t, err)
	return paths
}

// goldenPaths pins the composed context's file set
const goldenPaths = `Dockerfile
dist/ccboxtools
docker/desktop/home/.config/autostart/guiapp-autostart.desktop
docker/desktop/home/.config/google-chrome-for-testing/First Run
docker/desktop/home/.config/mimeapps.list
docker/desktop/home/.config/xfce4/helpers.rc
docker/desktop/home/.config/xfce4/xfconf/xfce-perchannel-xml/xfce4-desktop.xml
docker/desktop/home/.config/xfce4/xfconf/xfce-perchannel-xml/xfce4-panel.xml
docker/desktop/share/applications/web-browser.desktop
docker/desktop/share/xfce4/helpers/web-browser.desktop
docker/desktop.sh
docker/mise/config.toml
docker/mise/mise.lock
docker/web-browser
`
