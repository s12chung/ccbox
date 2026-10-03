// Package npm installs a package from the npm registry with the image's system
// npm: a complete Pkger of its own, not a download-and-place composite.
package npm

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/ccboxtools/pkg/util/httputil"
)

// npmRegistry is the registry npm sources packages and dist-tags from.
const npmRegistry = "https://registry.npmjs.org"

// manifest is the slice of an npm dist-tag document we read.
type manifest struct {
	Version string `json:"version"`
}

// latestAt returns the version npm's "latest" dist-tag points at for the package:
// the registry serves that tag's manifest at /<pkg>/latest; we read its version.
func latestAt(registry, pkg string) (string, error) {
	url := fmt.Sprintf("%s/%s/latest", registry, pkg)
	resp, err := httputil.GetOK(nil, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck // failing is ok

	var m manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return "", err
	}
	if m.Version == "" {
		return "", fmt.Errorf("npm registry %s: no version in manifest", url)
	}
	return m.Version, nil
}

// Npm installs a package from the npm registry with the image's system npm.
type Npm struct {
	pkginfo.Npm

	name string
}

// New returns the npm install for the CLI's name and package.
func New(name string, pkg pkginfo.Npm) Npm { return Npm{Npm: pkg, name: name} }

// Name is the CLI's name.
func (n Npm) Name() string { return n.name }

// Latest returns the version npm's "latest" dist-tag points at for the package.
func (n Npm) Latest() (string, error) {
	return latestAt(npmRegistry, n.Package)
}

// Install runs npm's global install with dir as the prefix, so the package tree
// and its bin land inside it. The package's bin name is assumed to be the CLI's.
func (n Npm) Install(dir, version string) error {
	args := []string{
		"install", "-g", "--prefix", dir,
		// npm runs lifecycle scripts by default (mise's backend skips them, which
		// is why the old image build needed --ignore-scripts=false); the allowlist
		// entry keeps npm 11 from warning per install.
		"--ignore-scripts=false", "--allow-scripts=" + n.Package,
		n.Package + "@" + version,
	}
	if err := runCmd("npm", args...); err != nil {
		return fmt.Errorf("npm install %s@%s: %w", n.Package, version, err)
	}
	return nil
}

// RelBin is npm's global bin layout under a --prefix.
func (n Npm) RelBin() string { return "bin/" + n.name }

// runCmd runs a command with output passed through; swapped in tests.
var runCmd = func(name string, args ...string) error {
	//nolint:gosec // npm is fixed and args are ours (the package name is firm-validated)
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
