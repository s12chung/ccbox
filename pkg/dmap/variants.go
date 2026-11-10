package dmap

import (
	"github.com/s12chung/ccbox/pkg/util/slug"
)

// variantSuffix is the run's variant's suffix — on the devbox name and the log
// files; the proxied headless variant's is empty
func variantSuffix(serveVNC, noProxy bool) string {
	suffix := ""
	if serveVNC {
		suffix = "-vnc"
		if !noProxy {
			suffix += "-proxy" // the egress wall makes the proxied vnc combo its own devbox
		}
	} else if noProxy {
		suffix = "-plain"
	}
	return suffix
}

// devboxName is the devbox's fixed name: the ccbox prefix (keeping the project slug's
// leading dash valid) plus its variant suffix — one devbox per variant
func devboxName(projectDir string, serveVNC, noProxy bool) string {
	return "ccbox" + slug.Path(projectDir) + variantSuffix(serveVNC, noProxy)
}
