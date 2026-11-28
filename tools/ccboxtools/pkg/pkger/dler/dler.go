// Package dler resolves a CLI's latest version from its release channel and
// downloads the version's file: the two modes of a release-URL download source.
package dler

import (
	"fmt"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

// guardVersion refuses anything but an exact version — an error page or HTML
// must not become the installed version.
func guardVersion(url, v string) (string, error) {
	if !pkginfo.IsPinned(v) {
		return "", fmt.Errorf("%s: %q is not a version", url, v)
	}
	return v, nil
}
