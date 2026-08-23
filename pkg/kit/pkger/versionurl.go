package pkger

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/s12chung/ccbox/pkg/util/httputil"
	"github.com/s12chung/ccbox/pkg/util/log"
)

// VersionURL pins via a URL whose body is a bare version — xAI's channel
// endpoints, e.g. https://x.ai/cli/stable -> 1.0.5. The download URL templates
// carry a literal $version, substituted by the image build after pinning.
type VersionURL struct {
	URL           string `yaml:"url"`
	LinuxX64URL   string `yaml:"linux_x64_url"`
	LinuxArm64URL string `yaml:"linux_arm64_url"`
}

// versionRe guards the pin: a version endpoint serves a bare semver, so anything
// else (an error page, HTML) must not become CLI_VERSION.
var versionRe = regexp.MustCompile(`^\d+(\.\d+)*([-+].+)?$`)

// Latest returns the bare version the endpoint serves.
func (u VersionURL) Latest() (string, error) {
	return versionAt(u.URL)
}

// Arg renders the versionurl scheme of the PKGER build arg: the pin endpoint
// followed by the per-platform download templates, pipe-joined.
func (u VersionURL) Arg() string {
	return "versionurl:" + strings.Join([]string{u.URL, u.LinuxX64URL, u.LinuxArm64URL}, "|")
}

func versionAt(url string) (string, error) {
	resp, err := httputil.Get(context.Background(), url)
	if err != nil {
		return "", err
	}
	defer func() { log.WarnErr("close version response", resp.Body.Close()) }()
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
