package pkger

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/s12chung/ccbox/pkg/log"
)

// VersionURL pins via a URL whose body is a bare version — xAI's channel
// endpoints, e.g. https://x.ai/cli/stable -> 1.0.5.
type VersionURL struct {
	URL string
}

// versionRe guards the pin: a version endpoint serves a bare semver, so anything
// else (an error page, HTML) must not become CLI_VERSION.
var versionRe = regexp.MustCompile(`^\d+(\.\d+)*([-+].+)?$`)

// Latest returns the bare version the endpoint serves.
func (u VersionURL) Latest() (string, error) {
	return versionAt(u.URL)
}

func versionAt(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer log.Defer("close version response", resp.Body.Close)
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
