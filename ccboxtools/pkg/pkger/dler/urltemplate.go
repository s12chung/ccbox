package dler

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/ccboxtools/pkg/util/httputil"
)

// arm64Arch is the arm64 name the vendor urls and goarch share.
const arm64Arch = "arm64"

// URLTemplate downloads the version's file by substituting it into the config's
// per-arch url templates: the template mode of the download source. The url's
// body is the bare version, and the file is served as-is, unverified.
type URLTemplate struct {
	pkginfo.DownloadTemplate

	url string
	// client fetches through instead of the default client — nil; the test TLS
	// server injects its own
	client *http.Client
}

// NewURLTemplate builds the template mode's downloader.
func NewURLTemplate(url string, tpl pkginfo.DownloadTemplate) URLTemplate {
	return URLTemplate{url: url, DownloadTemplate: tpl}
}

// Latest fetches the url's body — its bare version — versionRe-guarded.
func (u URLTemplate) Latest() (string, error) {
	body, err := fetchBody(u.client, u.url)
	if err != nil {
		return "", err
	}
	return guardVersion(u.url, strings.TrimSpace(string(body)))
}

// Download fetches the version's file for the running arch; the caller closes it.
func (u URLTemplate) Download(version string) (io.ReadCloser, error) {
	url, err := u.downloadURL(version)
	if err != nil {
		return nil, err
	}

	//nolint:bodyclose // the caller closes: the FilePkger.Install's defer drops it
	resp, err := httputil.GetOK(u.client, url)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (u URLTemplate) downloadURL(version string) (string, error) {
	tmpl := ""
	switch runtime.GOARCH {
	case "amd64":
		tmpl = u.X64URL
	case arm64Arch:
		tmpl = u.Arm64URL
	}
	if tmpl == "" {
		return tmpl, fmt.Errorf("dler: no download url for %s", runtime.GOARCH)
	}
	return strings.ReplaceAll(tmpl, "$version", version), nil
}

// subArch substitutes $arch in url with goarch's vendor name (x64/arm64); a url
// without $arch passes through.
func subArch(url, goarch string) string {
	arch, ok := map[string]string{"amd64": "x64", arm64Arch: arm64Arch}[goarch]
	if !ok {
		return url
	}
	return strings.ReplaceAll(url, "$arch", arch)
}
