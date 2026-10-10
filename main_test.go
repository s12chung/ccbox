package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/dockerfile"
	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// TestEmbedMiseImage guards the Dockerfile/GenerateLock coupling: cmd's build musts
// the `FROM <image> AS mise` parse, so the Dockerfile tmpl must carry it.
func TestEmbedMiseImage(t *testing.T) {
	_, err := mise.ImageFromDockerfile(dockerfile.Template())
	require.NoError(t, err)
}

// TestBuildContextHasCopySources guards the context/Dockerfile coupling: every path
// the Dockerfile COPYs from the build context must be present in the context that
// gets built — the embed as readied by mise.BuildFS + dockerfile.PatchFS, which
// inject what the embed no longer carries (the system config, the Dockerfile).
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

// TestBuildContext_Golden pins the full composed build context to its manifest
// golden — every file `ccbox build` ships, embedded or injected. dist/ccboxtools
// is `make build`'s output, so it's pinned to presence, not bytes.
func TestBuildContext_Golden(t *testing.T) {
	src := composedContext(t)
	assert.Equal(t, goldenManifest, strings.Join(contextManifest(t, src), "\n")+"\n")
}

// composedContext assembles the build context the way `ccbox build` does: the
// embed readied by mise.BuildFS — the seeded default config + a stub lock — and
// dockerfile.PatchFS's Dockerfile render.
func composedContext(t *testing.T) fs.FS {
	t.Helper()
	testutil.Home(t) // keeps the test off the host's user level
	require.NoError(t, mise.SeedConfig(mise.UserConfigPath()))
	require.NoError(t, os.WriteFile(mise.LockPath(mise.UserConfigPath()), []byte("lock"), osutil.File))
	src, err := mise.BuildFS(embedBuildContext, t.TempDir())
	require.NoError(t, err)
	src, err = dockerfile.PatchFS(src)
	require.NoError(t, err)
	return src
}

// contextManifest lists the composed context's files as path→sha256 lines, in the
// walk's lexical order
func contextManifest(t *testing.T, src fs.FS) []string {
	t.Helper()
	var lines []string
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		body, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		if p == "dist/ccboxtools" {
			lines = append(lines, p+"\t<built>")
			return nil
		}
		sum := sha256.Sum256(body)
		lines = append(lines, p+"\t"+hex.EncodeToString(sum[:]))
		return nil
	})
	require.NoError(t, err)
	return lines
}

// goldenManifest pins the composed context's path→sha256 manifest; dist/ccboxtools
// is pinned to presence via the <built> marker
const goldenManifest = `Dockerfile	4a0b1bbc556015f3ba62b84a85b62c1c55f6837bdd6e311d22f3b0377a9c1c3f
dist/ccboxtools	<built>
docker/desktop/home/.config/autostart/guiapp-autostart.desktop	2c28d1f248019794c51a34926013214944010e4b7765e126f981d4b7df3b6f94
docker/desktop/home/.config/google-chrome-for-testing/First Run	01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b
docker/desktop/home/.config/mimeapps.list	d93b70b830f8b2d2c5fa2f5d1e16f01a600a2253d63ef45abd997fd8e0e95b67
docker/desktop/home/.config/xfce4/helpers.rc	03770a71f94ce41a70b6fa8cc11e658b9c75441879814361830fdb00eaac2e4e
docker/desktop/home/.config/xfce4/xfconf/xfce-perchannel-xml/xfce4-desktop.xml	43c6b81a0aedd4a5355ddc8bc3404544fa5ef746631a0b8a515e1a8a4229384a
docker/desktop/home/.config/xfce4/xfconf/xfce-perchannel-xml/xfce4-panel.xml	834413340e9d657c4c0c845e359195b3b657007683c70a24e99f47c6c1f391b8
docker/desktop/share/applications/web-browser.desktop	476c4d72c08e6c5ee724b30835d4480e6d1403c66751df9e35708bcc52b23491
docker/desktop/share/xfce4/helpers/web-browser.desktop	e2dc5f7cd09fc5cb7f93cc7c164f2cf9c58f4f6e42d44abb89a8d37ba424fbad
docker/desktop.sh	c907d6c615fd98f16d1fa036d09c625fc5b4ff59a2f6485f9d9e629ff84150fd
docker/mise/config.toml	fb6efbbf23dfae55b0c48a1faa08d25f06f28b056628984cf0f3a78155f4f341
docker/mise/mise.lock	0c030586945fe504b604ecc2e875c38ede400cd5cd73da9730302162e6b02c6f
docker/web-browser	8a48156c7b8a5cf68111190e7008e13259f5c7f1d94d7bed02c04086f50a38dd
`
