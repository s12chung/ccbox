package projectcfg

import (
	"slices"
)

// AllowlistDefaults are the egress domains DefaultsAlias expands to: the CLI-independent
// tooling domains.
func AllowlistDefaults() []string { return slices.Clone(sharedAllowlistDefaults) }

// sharedAllowlistDefaults are the CLI-independent egress domains.
var sharedAllowlistDefaults = []string{
	// mise (tool version manager): version lists + release metadata
	"mise.en.dev",
	"mise-versions.jdx.dev",
	"tuf-repo-cdn.sigstore.dev",

	// Node / npm
	"registry.npmjs.org",
	"registry.yarnpkg.com",
	"nodejs.org",

	// Playwright browser binaries (image bakes the system libs; browsers fetched per-project)
	"cdn.playwright.dev",
	"playwright.download.prss.microsoft.com",

	// Python
	"pypi.org",
	"pythonhosted.org",

	// Ruby (gems + from-source tarballs)
	"rubygems.org",
	"cache.ruby-lang.org",

	// Go (vanity imports, module proxy, checksum db, toolchain mirror)
	"golang.org",
	"proxy.golang.org",
	"sum.golang.org",
	"dl.google.com",
	"storage.googleapis.com",

	// GitHub: source + release assets (used by gh, delta, yq, rg, fd, jq, python-build-standalone, ruby-build)
	"github.com",
	"githubusercontent.com",
	"githubassets.com",

	// Man pages (canonical man text, not cheatsheets)
	"manpages.debian.org",
	"man7.org",
	"man.cx",
	"linux.die.net",
	"manpages.ubuntu.com",
}
