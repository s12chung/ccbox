package cfg

import (
	"slices"
)

// AllowlistDefaults are the egress domains DefaultsAlias expands to: the CLI-independent
// tooling domains.
func AllowlistDefaults() []string { return slices.Clone(sharedAllowlistDefaults) }

// sharedAllowlistDefaults are the CLI-independent egress domains; the runtimes' ride
// their own aliases.
var sharedAllowlistDefaults = []string{
	// mise (tool version manager): version lists + release metadata
	"jdx.dev",
	"mise.en.dev",
	"tuf-repo-cdn.sigstore.dev",

	// Playwright browser binaries (image bakes the system libs; browsers fetched per-project)
	"cdn.playwright.dev",
	"playwright.download.prss.microsoft.com",

	// GitHub: source + release assets
	"github.com",
	"githubusercontent.com",
	"githubassets.com",

	// Other repos: git hosts + Google's source viewer (vanity import sources)
	"cs.opensource.google",
	"gitlab.com",
	"bitbucket.org",
	"codeberg.org",
	"sourceforge.net",

	// Docker: Hub's registry/auth/CDN subdomains + the other common registries
	"docker.io",
	"docker.com",
	"ghcr.io",
	"quay.io",
	"gcr.io",
	"mcr.microsoft.com",
	"registry.k8s.io",

	// Man pages (canonical man text, not cheatsheets)
	"manpages.debian.org",
	"man7.org",
	"man.cx",
	"linux.die.net",
	"manpages.ubuntu.com",
}
