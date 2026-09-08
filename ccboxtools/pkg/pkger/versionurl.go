package pkger

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
)

// VersionURL installs from a URL whose body is a bare version — xAI's channel
// endpoints, e.g. https://x.ai/cli/stable -> 1.0.5. The download URL templates
// carry a literal $version, substituted at install.
type VersionURL struct {
	URL           string `json:"url"             yaml:"url"`
	LinuxX64URL   string `json:"linux_x64_url"   yaml:"linux_x64_url"`
	LinuxArm64URL string `json:"linux_arm64_url" yaml:"linux_arm64_url"`
}

func init() {
	// https endpoints; download templates may carry a literal $version
	https := rule.Match{Regexp: regexp.MustCompile(`^https://\S+$`)}
	firm.MustRegisterType(firm.NewDefinition[VersionURL]().Validates(firm.RuleMap{
		"URL":           {https},
		"LinuxX64URL":   {https},
		"LinuxArm64URL": {https},
	}))
}

// versionRe guards the pin: a version endpoint serves a bare semver, so anything
// else (an error page, HTML) must not become the installed version.
var versionRe = regexp.MustCompile(`^\d+(\.\d+)*([-+].+)?$`)

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
