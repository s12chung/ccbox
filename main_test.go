package main

import (
	"bufio"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// TestEmbedMiseImage guards the Dockerfile/GenerateLock coupling: cmd's initMiseImage
// musts the `FROM <image> AS mise` parse, so the embed's Dockerfile must carry it.
func TestEmbedMiseImage(t *testing.T) {
	dockerfile, err := fs.ReadFile(embedBuildContext, "Dockerfile")
	require.NoError(t, err)
	_, err = mise.ImageFromDockerfile(dockerfile)
	require.NoError(t, err)
}

// TestBuildContextHasCopySources guards the context/Dockerfile coupling: every path
// the Dockerfile COPYs from the build context must be present in the context that
// gets built — the embed as readied by mise.BuildFS, which injects the system
// config the embed no longer carries (pkg/mise/config.toml).
func TestBuildContextHasCopySources(t *testing.T) {
	testutil.Home(t) // BuildFS requires a committed config+lock: seed the default, keeping the test off the host's user level
	require.NoError(t, mise.SeedConfig(mise.UserConfigPath()))
	require.NoError(t, os.WriteFile(mise.LockPath(mise.UserConfigPath()), []byte("lock"), ioutil.File))
	src, err := mise.BuildFS(embedBuildContext, t.TempDir())
	require.NoError(t, err)

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
