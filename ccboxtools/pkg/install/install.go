// Package install ensures a CLI's current version is installed under the clis
// root: resolve latest, install if stale, expose it on PATH, prune old versions.
package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkger"
)

// Run resolves pkgDir's latest version, installs it unless it is already current,
// and prunes every other version of the CLI. A channel failure falls back to the
// installed version; with nothing installed, it fails.
func Run(pkgDir pkger.PkgDir) error {
	active := currentVersion(pkgDir)
	latest, err := pkgDir.Latest()
	if err != nil {
		if active == "" {
			return fmt.Errorf("resolve %s latest: %w", pkgDir.Name(), err)
		}
		log.Warnf("resolving %s latest cli failed (%v); using installed %s", pkgDir.Name(), err, active)
		return nil
	}
	if latest == active {
		log.Infof("%s %s is up to date", pkgDir.Name(), latest)
		return nil
	}

	if err := install(pkgDir, latest); err != nil {
		return err
	}
	log.Infof("installed %s %s", pkgDir.Name(), latest)
	return prune(pkgDir.Dir(), latest)
}

// install puts version in place: download to a temp dir, rename in, then flip
// the current and bin symlinks.
func install(pkgDir pkger.PkgDir, version string) error {
	tmp := pkgDir.TmpDir(version)
	if err := safeMkdir(tmp); err != nil {
		return err
	}
	if err := pkgDir.Install(tmp, version); err != nil {
		return fmt.Errorf("install %s %s: %w", pkgDir.Name(), version, err)
	}
	if err := safeMv(tmp, pkgDir.VersionDir(version)); err != nil {
		return err
	}
	return updateSymlinks(pkgDir, version)
}

// updateSymlinks flips current and the PATH-exposed bin link to version.
func updateSymlinks(pkgDir pkger.PkgDir, version string) error {
	if err := replaceSymlink(version, pkgDir.Current()); err != nil {
		return err
	}
	return replaceSymlink(pkgDir.BinLinkTarget(), pkgDir.BinLink())
}

// currentVersion reads the version dir the pkg's current link points at, "" when unset.
func currentVersion(pkgDir pkger.PkgDir) string {
	target, err := os.Readlink(pkgDir.Current())
	if err != nil {
		return ""
	}
	return filepath.Base(target)
}
