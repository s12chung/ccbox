package dmap

import (
	"github.com/s12chung/ccbox/pkg/util/slug"
)

// variantSuffix is the run's variant's suffix — on the devbox name and the log
// files; the proxied headless variant's is empty
func variantSuffix(serveVNC, isProxied bool) string {
	suffix := ""
	if !isProxied {
		suffix += "-plain"
	}
	if serveVNC {
		suffix += "-vnc"
	}
	return suffix
}

// devboxName is the devbox's fixed name: the ccbox prefix (keeping the project slug's
// leading dash valid) plus its variant suffix — one devbox per variant
func devboxName(projectDir string, serveVNC, isProxied bool) string {
	return "ccbox" + slug.Path(projectDir) + variantSuffix(serveVNC, isProxied)
}

// ProxiedDevboxNames returns both proxied variants' devbox names
func ProxiedDevboxNames(projectDir string) []string {
	return []string{devboxName(projectDir, false, true), devboxName(projectDir, true, true)}
}
