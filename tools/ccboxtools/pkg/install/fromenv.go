package install

import (
	"fmt"
	"os"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/flock"
)

// ClisDirEnv overrides the clis root; the image mounts the global volume at the default.
const ClisDirEnv = "CCBOX_CLIS_DIR"

// EnvPkgDir returns the CLI the run's env names at the clis root — a boot
// without one fails.
func EnvPkgDir() (pkger.PkgDir, error) {
	pkginfoJSON := os.Getenv(pkginfo.EnvVar)
	if pkginfoJSON == "" {
		return pkger.PkgDir{}, fmt.Errorf("%s names no cli", pkginfo.EnvVar)
	}
	info, err := pkginfo.FromJSON(pkginfoJSON)
	if err != nil {
		return pkger.PkgDir{}, err
	}
	root := os.Getenv(ClisDirEnv)
	if root == "" {
		root = DefaultRoot
	}
	p, err := pkger.ForPkgInfo(info)
	if err != nil {
		return pkger.PkgDir{}, err
	}
	return pkger.PkgDir{Pkger: p, Root: root}, nil
}

// Hold holds the bin link's version for the container's life, so other
// devboxes' prunes can't delete it under a running session; returns the release.
func Hold(pkgDir pkger.PkgDir) (func() error, error) {
	version, err := pkgDir.LinkedVersion()
	if err != nil {
		return nil, err
	}
	return flock.Hold(pkgDir.VersionDir(version))
}
