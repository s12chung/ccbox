package pkginfo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
)

// Npm installs from the npm registry's "latest" dist-tag.
type Npm struct {
	Package string `json:"package" yaml:"package"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[Npm]().Validates(firm.RuleMap{
		"Package": {rule.Match{Regexp: regexp.MustCompile(`^(@[a-z0-9-]+/)?[a-z0-9][a-z0-9._-]*$`)}},
	}))
}

// manifest is the slice of an npm dist-tag document we read.
type manifest struct {
	Version string `json:"version"`
}

// latestAt returns the version npm's "latest" dist-tag points at for the package:
// the registry serves that tag's manifest at /<pkg>/latest; we read its version.
func latestAt(registry, pkg string) (string, error) {
	url := fmt.Sprintf("%s/%s/latest", registry, pkg)
	resp, err := httpGet(context.Background(), url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck // failing is ok
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("npm registry %s: %s", url, resp.Status)
	}

	var m manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return "", err
	}
	if m.Version == "" {
		return "", fmt.Errorf("npm registry %s: no version in manifest", url)
	}
	return m.Version, nil
}
