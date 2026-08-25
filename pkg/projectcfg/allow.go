package projectcfg

import "github.com/s12chung/ccbox/pkg/harness"

// allowDefaults are the egress domains DefaultsToken expands to: the wall's built-in
// allow — shared defaults plus every supported CLI's own domains, computed per call
// from the loaded cli set.
func allowDefaults() []string {
	return append(append([]string{}, sharedAllowDefaults...), cliAllowDomains()...)
}

// sharedAllowDefaults are the CLI-independent egress domains.
var sharedAllowDefaults = []string{
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

// cliAllowDomains concatenates every supported CLI's own egress domains, in All's order.
func cliAllowDomains() []string {
	var domains []string
	for _, c := range harness.All() {
		domains = append(domains, c.AllowDomains...)
	}
	return domains
}

// expandAllowlist replaces each DefaultsToken with allowDefaults. A nil list (allowlist
// unset) falls back to the built-ins; an explicit empty list ([]) stays empty, so the wall
// allows nothing.
func expandAllowlist(domains []string) []string {
	if domains == nil {
		domains = []string{DefaultsToken}
	}
	var out []string
	for _, d := range domains {
		if d == DefaultsToken {
			out = append(out, allowDefaults()...)
			continue
		}
		out = append(out, d)
	}
	return out
}
