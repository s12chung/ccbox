package dler

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/httputil"
)

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
	body, err := httputil.Body(u.client, u.url)
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
	case "arm64":
		tmpl = u.Arm64URL
	}
	if tmpl == "" {
		return tmpl, fmt.Errorf("dler: no download url for %s", runtime.GOARCH)
	}
	return strings.ReplaceAll(tmpl, "$version", version), nil
}
