package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/cfg"
	"github.com/s12chung/ccbox/pkg/kit/globkit"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/printutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestAliasSections(t *testing.T) {
	cliName := "pi"
	config = &cfg.Config{CLIName: cliName}
	t.Cleanup(func() { config = nil })

	assert.Equal(t, []string{
		"# ccbox-defaults:",
		"#   tmpfs_masks:",
		"#     - .idea",
		"#     - .vscode",
		"#     - .zed",
		"#   volume_masks:",
		"#     - node_modules",
		"#     - .venv",
		"#     - target",
		"#   read_only_globs:",
		"#     - .ccbox.yaml",
		"#     - .env",
		"#     - **/*.pem",
		"#     - .env.*",
		"#     - .envrc",
		"#     - secrets",
		"#     - **/*.key",
		"#     - **/id_rsa*",
		"#   proxy.allowlist:",
		"#     - jdx.dev",
		"#     - mise.en.dev",
		"#     - tuf-repo-cdn.sigstore.dev",
		"#     - cdn.playwright.dev",
		"#     - playwright.download.prss.microsoft.com",
		"#     - github.com",
		"#     - githubusercontent.com",
		"#     - githubassets.com",
		"#     - cs.opensource.google",
		"#     - gitlab.com",
		"#     - bitbucket.org",
		"#     - codeberg.org",
		"#     - sourceforge.net",
		"#     - docker.io",
		"#     - docker.com",
		"#     - ghcr.io",
		"#     - quay.io",
		"#     - gcr.io",
		"#     - mcr.microsoft.com",
		"#     - registry.k8s.io",
		"#     - deb.debian.org",
		"#     - archive.debian.org",
		"#     - manpages.ubuntu.com",
		"#     - manpages.debian.org",
		"#     - man7.org",
		"#     - man.cx",
		"#     - linux.die.net",
		"",
		"# ccbox-set-harness (cli pi; a --vnc load adds the desktop's GUI app):",
		"#   - api.anthropic.com",
		"#   - api.openai.com",
		"#   - auth.openai.com",
		"#   - openrouter.ai",
		"#   - x.ai",
		"#   - api.z.ai",
		"#   - pi.dev",
		"#   - radius.pi.dev",
		"",
		"# ccbox-anthropic-provider:",
		"#   - api.anthropic.com",
		"",
		"# ccbox-openai-provider:",
		"#   - api.openai.com",
		"#   - auth.openai.com",
		"",
		"# ccbox-openrouter-provider:",
		"#   - openrouter.ai",
		"",
		"# ccbox-xai-provider:",
		"#   - x.ai",
		"",
		"# ccbox-zai-provider:",
		"#   - api.z.ai",
		"",
		"# ccbox-go-runtime:",
		"#   - golang.org",
		"#   - proxy.golang.org",
		"#   - sum.golang.org",
		"#   - dl.google.com",
		"#   - storage.googleapis.com",
		"",
		"# ccbox-node-runtime:",
		"#   - registry.npmjs.org",
		"#   - registry.yarnpkg.com",
		"#   - nodejs.org",
		"",
		"# ccbox-python-runtime:",
		"#   - pypi.org",
		"#   - pythonhosted.org",
		"",
		"# ccbox-rust-runtime:",
		"#   - sh.rustup.rs",
		"#   - static.rust-lang.org",
		"#   - crates.io",
		"#   - index.crates.io",
		"#   - static.crates.io",
	}, printutil.Render(aliasSections()))
}

func TestConfirmReadOnlyPaths(t *testing.T) {
	// writeDotEnv seeds n subdirs, each holding a .env for the **/.env glob to match
	writeDotEnv := func(t *testing.T, dir string, n int) {
		t.Helper()
		for i := range n {
			d := filepath.Join(dir, fmt.Sprintf("svc%02d", i))
			require.NoError(t, os.MkdirAll(d, osutil.Dir))
			require.NoError(t, os.WriteFile(filepath.Join(d, ".env"), []byte("K=v"), osutil.File))
		}
	}
	// loadReadOnlyConfig loads config over dir, whose .ccbox.yaml carries the glob —
	// testutil.Home keeps the user-level layer out of the match count
	loadReadOnlyConfig := func(t *testing.T, dir string) {
		t.Helper()
		testutil.Home(t)
		body := "cli: claude\nread_only_globs:\n  - \"**/.env\"\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".ccbox.yaml"), []byte(body), osutil.File))
		c, err := cfg.Load(dir, cfg.Flags{})
		require.NoError(t, err)
		config = c
		t.Cleanup(func() { config = nil })
	}

	t.Run("under the limit prints without asking", func(t *testing.T) {
		dir := t.TempDir()
		writeDotEnv(t, dir, 2)
		loadReadOnlyConfig(t, dir)
		assert.True(t, confirmReadOnlyPaths())
	})

	t.Run("at the limit a decline aborts", func(t *testing.T) {
		dir := t.TempDir()
		writeDotEnv(t, dir, globkit.MatchLimit)
		loadReadOnlyConfig(t, dir)
		testutil.Stdin(t, "")
		assert.False(t, confirmReadOnlyPaths())
	})

	t.Run("at the limit a confirm prints", func(t *testing.T) {
		dir := t.TempDir()
		writeDotEnv(t, dir, globkit.MatchLimit)
		loadReadOnlyConfig(t, dir)
		testutil.Stdin(t, "y\n")
		assert.True(t, confirmReadOnlyPaths())
	})
}
