package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/util/printutil"
)

func TestAliasSections(t *testing.T) {
	cliName := "pi"
	projectCfg = &projectcfg.Config{CLIName: &cliName}
	t.Cleanup(func() { projectCfg = nil })

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
		"#     - registry.npmjs.org",
		"#     - registry.yarnpkg.com",
		"#     - nodejs.org",
		"#     - cdn.playwright.dev",
		"#     - playwright.download.prss.microsoft.com",
		"#     - pypi.org",
		"#     - pythonhosted.org",
		"#     - rubygems.org",
		"#     - cache.ruby-lang.org",
		"#     - golang.org",
		"#     - proxy.golang.org",
		"#     - sum.golang.org",
		"#     - dl.google.com",
		"#     - storage.googleapis.com",
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
	}, printutil.Render(aliasSections()))
}
