// Package dler resolves a CLI's latest version from its release channel and
// downloads the version's file: the two modes of a release-URL download source.
package dler

import (
	"fmt"
	"regexp"
)

// versionRe guards the resolved version: anything else (an error page, HTML)
// must not become the installed version.
var versionRe = regexp.MustCompile(`^\d+(\.\d+)*([-+].+)?$`)

// guardVersion checks v against versionRe.
func guardVersion(url, v string) (string, error) {
	if !versionRe.MatchString(v) {
		return "", fmt.Errorf("%s: %q is not a version", url, v)
	}
	return v, nil
}
