package pkger

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/s12chung/ccbox/pkg/log"
)

// Npm pins via the npm registry's "latest" dist-tag.
type Npm struct {
	Package string
}

// npmRegistry is the registry Npm reads dist-tags from.
const npmRegistry = "https://registry.npmjs.org"

// manifest is the slice of an npm dist-tag document we read.
type manifest struct {
	Version string `json:"version"`
}

// Latest returns the version npm's "latest" dist-tag points at for the package:
// the registry serves that tag's manifest at /<pkg>/latest; we read its version.
func (n Npm) Latest() (string, error) {
	return latestAt(npmRegistry, n.Package)
}

// Arg renders the npm scheme of the PKGER build arg.
func (n Npm) Arg() string {
	return "npm:" + n.Package
}

func latestAt(registry, pkg string) (string, error) {
	url := fmt.Sprintf("%s/%s/latest", registry, pkg)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer log.Defer("close npm response", resp.Body.Close)
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
