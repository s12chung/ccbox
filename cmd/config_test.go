package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/kit/globkit"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/printutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestAliasSections(t *testing.T) {
	cliName := "pi"
	projectConfig = &projectcfg.Config{CLIName: &cliName}
	t.Cleanup(func() { projectConfig = nil })

	assert.Equal(t, []string{
		"# ccbox-defaults:",
		"#   tmpfs_masks:",
		"#     - .idea",
		"#     - .vscode",
		"#   volume_masks:",
		"#     - node_modules",
		"#     - .venv",
		"#     - vendor/bundle",
		"#   read_only_globs:",
		"#     - .ccbox.yaml",
		"#     - .ccbox.local.yaml",
		"#     - .env",
		"#     - .env.*",
		"#     - .envrc",
		"#     - secrets",
		"#     - **/*.pem",
		"#     - **/*.key",
		"#   allowlist:",
		"#     - mise.en.dev",
		"#     - mise-versions.jdx.dev",
		"#     - tuf-repo-cdn.sigstore.dev",
		"#     - cdn.playwright.dev",
		"#     - playwright.download.prss.microsoft.com",
		"#     - github.com",
		"#     - githubusercontent.com",
		"#     - githubassets.com",
		"#     - manpages.debian.org",
		"#     - man7.org",
		"#     - man.cx",
		"#     - linux.die.net",
		"#     - manpages.ubuntu.com",
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
		"# ccbox-ruby-runtime:",
		"#   - rubygems.org",
		"#   - cache.ruby-lang.org",
	}, printutil.Render(aliasSections()))
}

func TestConfirmReadOnlyPaths(t *testing.T) {
	// writeDotEnv seeds n subdirs, each holding a .env for the **/.env glob to match
	writeDotEnv := func(t *testing.T, dir string, n int) {
		t.Helper()
		for i := range n {
			d := filepath.Join(dir, fmt.Sprintf("svc%02d", i))
			require.NoError(t, os.MkdirAll(d, ioutil.Dir))
			require.NoError(t, os.WriteFile(filepath.Join(d, ".env"), []byte("K=v"), ioutil.File))
		}
	}
	// loadReadOnlyConfig loads projectConfig over dir, whose flags carry the glob —
	// testutil.Home keeps the user-level layer out of the match count
	loadReadOnlyConfig := func(t *testing.T, dir string) {
		t.Helper()
		testutil.Home(t)
		cliName := "claude"
		c, err := projectcfg.Load(dir, projectcfg.Config{CLIName: &cliName, ReadOnlyGlobs: []string{"**/.env"}}, false)
		require.NoError(t, err)
		projectConfig = c
		t.Cleanup(func() { projectConfig = nil })
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
