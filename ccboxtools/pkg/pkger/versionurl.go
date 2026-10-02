package pkger

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime"
	"strings"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

// versionRe guards the resolved version: a version endpoint serves a bare semver,
// so anything else (an error page, HTML) must not become the installed version.
var versionRe = regexp.MustCompile(`^\d+(\.\d+)*([-+].+)?$`)

// versionAt returns the bare version the endpoint serves.
func versionAt(url string) (string, error) {
	resp, err := httpGet(context.Background(), url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck // failing is ok
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(body))
	if !versionRe.MatchString(v) {
		return "", fmt.Errorf("%s: %q is not a version", url, v)
	}
	return v, nil
}

// VersionURL downloads a raw executable whose version is resolved from a URL.
type VersionURL struct{ pkginfo.VersionURL }

// Latest returns the bare version the endpoint serves.
func (u VersionURL) Latest() (string, error) {
	return versionAt(u.URL)
}

// Download fetches the version's binary for the running arch; the caller closes it.
func (u VersionURL) Download(version string) (io.ReadCloser, error) {
	tpl, err := u.template()
	if err != nil {
		return nil, err
	}

	resp, err := httpGet(context.Background(), strings.Replace(tpl, "$version", version, 1))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close() //nolint:errcheck // failing is ok
		return nil, fmt.Errorf("%s: %s", resp.Request.URL, resp.Status)
	}
	return resp.Body, nil
}

// template picks the download URL template for the running arch.
func (u VersionURL) template() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return u.LinuxX64URL, nil
	case "arm64":
		return u.LinuxArm64URL, nil
	default:
		return "", fmt.Errorf("pkger: no download url for %s", runtime.GOARCH)
	}
}
