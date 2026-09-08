package pkger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

// execFileMode is the mode installed executables carry (mirrors the host's ioutil.ExecFile).
const execFileMode os.FileMode = 0o755

// VersionURL downloads a raw executable whose version is resolved from a URL.
type VersionURL struct {
	pkginfo.VersionURL

	name string
}

// Name is the CLI's name.
func (u VersionURL) Name() string { return u.name }

// Latest returns the bare version the endpoint serves.
func (u VersionURL) Latest() (string, error) {
	return versionAt(u.URL)
}

// Install downloads the version's binary for the running arch into dir.
func (u VersionURL) Install(dir, version string) error {
	tpl, err := u.template()
	if err != nil {
		return err
	}

	resp, err := httpGet(context.Background(), strings.Replace(tpl, "$version", version, 1))
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // failing is ok
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", resp.Request.URL, resp.Status)
	}

	path := filepath.Join(dir, u.name)
	//nolint:gosec // path is the clis root + the firm-validated CLI name
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, execFileMode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		return errors.Join(err, file.Close())
	}
	return errors.Join(file.Chmod(execFileMode), file.Close())
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

// RelBin is the raw binary dropped at the install dir's root.
func (u VersionURL) RelBin() string { return u.name }
