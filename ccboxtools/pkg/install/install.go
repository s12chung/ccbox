// Package install ensures a CLI's current version is installed under the clis
// root: resolve latest, install if stale, expose it on PATH, prune old versions.
package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/s12chung/ccbox/ccboxtools/pkg/lock"
	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkger"
)

// Run resolves pkgDir's latest version, installs it unless it is already current,
// and prunes every other version of the CLI — all under the CLI's install lock.
// A Run() failure keeps the installed version; with nothing installed, it fails.
func Run(pkgDir pkger.PkgDir) error {
	return lock.Do(pkgDir.LockPath(), func() error {
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
		log.Infof("installing %s %s", pkgDir.Name(), latest)

		if err := install(pkgDir, latest); err != nil {
			return err
		}
		log.Infof("installed %s %s", pkgDir.Name(), latest)
		return prune(pkgDir.Dir(), latest)
	}, skipInstalled(pkgDir))
}

// skipInstalled reports whether an installed version can stand in while another
// container updates; LockWaitEnv turns the skip off, forcing the wait.
func skipInstalled(pkgDir pkger.PkgDir) func() bool {
	return func() bool {
		if os.Getenv(LockWaitEnv) != "" {
			return false
		}
		active := currentVersion(pkgDir)
		if active == "" {
			return false
		}
		log.Infof("%s update in flight; using installed %s", pkgDir.Name(), active)
		return true
	}
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
