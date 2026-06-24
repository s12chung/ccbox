package docker

import (
	"fmt"
	"regexp"
	"strings"
)

// allowFileName is the tinyproxy filter file, generated and copied into tinyproxyDir.
const allowFileName = "allow.txt"

// allowDefaults are the domains the egress wall permits by default, enabled via
// .ccbox.yaml `allowlist.defaults`. Stored as bare domains; renderAllow turns each
// into a filter regex matching the apex and any subdomain.
var allowDefaults = []string{
	// mise (tool version manager): version lists + release metadata
	"mise.en.dev",
	"mise-versions.jdx.dev",

	// Node / npm
	"registry.npmjs.org",
	"registry.yarnpkg.com",
	"nodejs.org",

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

	// Anthropic / Claude Code
	"platform.claude.com",
	"api.anthropic.com",
	"mcp-proxy.anthropic.com",
	"statsig.anthropic.com",
	"sentry.io",
}

// AllowOverride builds the embedfs override that seeds the wall's allow.txt: the
// rendered defaults (when enabled) plus domains.
func AllowOverride(defaults bool, domains []string) map[string][]byte {
	all := domains
	if defaults {
		all = append(append([]string{}, allowDefaults...), domains...)
	}
	return map[string][]byte{allowFileName: renderAllow(all)}
}

// renderAllow turns each bare domain into an ERE filter line ((^|\.)domain$),
// matching the apex and any subdomain.
func renderAllow(domains []string) []byte {
	var b strings.Builder
	for _, d := range domains {
		fmt.Fprintf(&b, "(^|\\.)%s$\n", regexp.QuoteMeta(d))
	}
	return []byte(b.String())
}
