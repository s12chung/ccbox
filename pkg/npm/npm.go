// Package npm resolves package versions against the npm registry.
package npm

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/s12chung/ccbox/pkg/log"
)

const registry = "https://registry.npmjs.org"

// manifest is the slice of an npm version document we read.
type manifest struct {
	Version string `json:"version"`
}

// latestURL is the registry manifest for pkg's "latest" dist-tag.
func latestURL(pkg string) string {
	return fmt.Sprintf("%s/%s/latest", registry, pkg)
}

// LatestVersion returns the version npm's "latest" dist-tag points at for pkg.
// The registry serves that tag's manifest at /<pkg>/latest; we read its version.
func LatestVersion(pkg string) (string, error) {
	resp, err := http.Get(latestURL(pkg))
	if err != nil {
		return "", err
	}
	defer log.Defer("close npm response", resp.Body.Close)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("npm registry %s: %s", latestURL(pkg), resp.Status)
	}

	var m manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return "", err
	}
	if m.Version == "" {
		return "", fmt.Errorf("npm registry %s: no version in manifest", latestURL(pkg))
	}
	return m.Version, nil
}
