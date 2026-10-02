package pkger

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"runtime"
	"strings"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

// versionRe guards the resolved version: a bare semver from the endpoint's
// body — anything else (an error page, HTML)
// must not become the installed version.
var versionRe = regexp.MustCompile(`^\d+(\.\d+)*([-+].+)?$`)

// VersionURL downloads a raw executable whose version is resolved from a URL's
// body by a Versioner.
type VersionURL struct {
	pkginfo.VersionURL

	versioner Versioner
}

// pkgerize returns the VersionURL for the pkginfo.VersionURL
func pkgerize(vu pkginfo.VersionURL) (VersionURL, error) {
	if vu.DownloadTemplate == nil { // unrepresentable via firm-validated config; Go-constructed configs hit it
		return VersionURL{}, errors.New("pkger: no download_template")
	}

	return VersionURL{VersionURL: vu, versioner: TextVersioner{}}, nil
}

// Latest fetches the url's body and returns the versioner's version,
// versionRe-guarded.
func (u VersionURL) Latest() (string, error) {
	body, err := httpBody(u.URL)
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
func (u VersionURL) Download(version string) (io.ReadCloser, error) {
	url, err := u.downloadURL(version)
	if err != nil {
		return nil, err
	}

	resp, err := httpGetOK(url)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (u VersionURL) downloadURL(version string) (string, error) {
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
