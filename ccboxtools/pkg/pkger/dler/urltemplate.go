package dler

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"runtime"
	"strings"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/ccboxtools/pkg/util/httputil"
)

// versionRe guards the resolved version: a bare semver from the endpoint's
// body — anything else (an error page, HTML)
// must not become the installed version.
var versionRe = regexp.MustCompile(`^\d+(\.\d+)*([-+].+)?$`)

// URLTemplate downloads a raw executable whose version is resolved from a URL's
// body by a Versioner.
type URLTemplate struct {
	pkginfo.ReleaseURL

	versioner Versioner
}

// New returns the URLTemplate for the pkginfo.ReleaseURL
func New(vu pkginfo.ReleaseURL) (URLTemplate, error) {
	if vu.DownloadTemplate == nil { // unrepresentable via firm-validated config; Go-constructed configs hit it
		return URLTemplate{}, errors.New("dler: no download_template")
	}

	return URLTemplate{ReleaseURL: vu, versioner: TextVersioner{}}, nil
}

// Latest fetches the url's body and returns the versioner's version,
// versionRe-guarded.
func (u URLTemplate) Latest() (string, error) {
	body, err := httputil.Body(nil, u.URL)
	if err != nil {
		return "", err
	}

	v, err := u.versioner.Latest(body)
	if err != nil {
		return "", err
	}
	if !versionRe.MatchString(v) {
		return "", fmt.Errorf("%s: %q is not a version", u.URL, v)
	}
	return v, nil
}

// Download fetches the version's binary for the running arch; the caller closes it.
func (u URLTemplate) Download(version string) (io.ReadCloser, error) {
	url, err := u.downloadURL(version)
	if err != nil {
		return nil, err
	}

	resp, err := httputil.GetOK(nil, url)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (u URLTemplate) downloadURL(version string) (string, error) {
	tmpl := ""
	switch runtime.GOARCH {
	case "amd64":
		tmpl = u.DownloadTemplate.X64URL
	case "arm64":
		tmpl = u.DownloadTemplate.Arm64URL
	}
	if tmpl == "" {
		return tmpl, fmt.Errorf("pkger: no download url for %s", runtime.GOARCH)
	}
	return strings.ReplaceAll(tmpl, "$version", version), nil
}
