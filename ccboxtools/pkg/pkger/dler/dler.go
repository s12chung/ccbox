// Package dler resolves a CLI's latest version from its release channel and
// downloads the version's file.
package dler

import (
	"fmt"
	"net/http"
	"regexp"
	"runtime"

	"github.com/s12chung/ccbox/ccboxtools/pkg/util/httputil"
)

// fetchBody GETs the url's body — $arch substituted.
func fetchBody(client *http.Client, url string) ([]byte, error) {
	return httputil.Body(client, subArch(url, runtime.GOARCH))
}

// versionRe guards the resolved version: a bare semver from the release url's
// body — anything else (an error page, HTML) must not become the installed
// version.
var versionRe = regexp.MustCompile(`^\d+(\.\d+)*([-+].+)?$`)

// guardVersion checks v against versionRe.
func guardVersion(url, v string) (string, error) {
	if !versionRe.MatchString(v) {
		return "", fmt.Errorf("%s: %q is not a version", url, v)
	}
	return v, nil
}
