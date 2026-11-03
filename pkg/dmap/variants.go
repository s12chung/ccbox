package dmap

import (
	"github.com/s12chung/ccbox/pkg/util/slug"
)

// The run's variant suffixes — on the devbox name, the image tag, and the log
// files. The plain and proxied variants share the image, so the tag only ever
// carries vncSuffix.
const (
	vncSuffix   = "-vnc"
	plainSuffix = "-plain"
)

// variantSuffix is the run's variant's suffix; the proxied headless variant's is empty
func variantSuffix(serveVNC, noProxy bool) string {
	switch {
	case serveVNC:
		return vncSuffix
	case noProxy:
		return plainSuffix
	}
	return ""
}

// devboxName is the devbox's fixed name: the ccbox prefix (keeping the project slug's
// leading dash valid) plus its variant suffix — one devbox per variant
func devboxName(projectDir string, serveVNC, noProxy bool) string {
	return "ccbox" + slug.Path(projectDir) + variantSuffix(serveVNC, noProxy)
}
