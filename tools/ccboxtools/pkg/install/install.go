// Package install ensures a CLI's current version is installed under the clis
// root: resolve latest, install if stale, expose it on PATH, prune old versions.
package install

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/flock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/installutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// DefaultRoot is the clis root's container mount: the ccbox-clis global volume's,
// which the PATH leads with its bin dir.
const DefaultRoot = "/opt/ccbox/clis"

// AppsRoot is the apps root's container mount: the ccbox-apps global volume's,
// where the GUI app (the VNC_CONFIG env's gui_app) installs at start.
const AppsRoot = "/opt/ccbox/apps"

// FromPkgDir resolves pkgDir's latest version, installs it unless it is already
// current, and prunes every other version of the CLI — all under the CLI's install
// lock. A failure keeps the installed version; with nothing usable installed, it fails.
func FromPkgDir(pkgDir pkger.PkgDir) error {
	return flock.Do(pkgDir.LockPath(), func() error {
		active := installutil.CurrentVersion(pkgDir.Current())
		latest, err := pkgDir.Latest()
		if err != nil {
			if active == "" {
				return fmt.Errorf("resolve %s latest: %w", pkgDir.Name(), err)
			}
			if !installed(pkgDir, active) {
				// the stand-in must carry the executable: an install predating the
				// layout can't serve the session, so surface the resolve failure
				return fmt.Errorf("resolving %s latest failed (%w) and installed %s lacks %s", pkgDir.Name(), err, active, pkgDir.RelBin())
			}
			log.Warnf("resolving %s latest cli failed (%v); using installed %s", pkgDir.Name(), err, active)
			return nil
		}
		if latest == active && installed(pkgDir, latest) {
			log.Infof("%s %s is up to date", pkgDir.Name(), latest)
			return nil
		}
		log.Infof("installing %s %s", pkgDir.Name(), latest)

		if err := install(pkgDir, latest); err != nil {
			return err
		}
		log.Infof("installed %s %s", pkgDir.Name(), latest)
		return installutil.Prune(pkgDir.Dir(), latest)
	}, skipInstalled(pkgDir))
}

// FromPkgInfo installs info's CLI or GUI app at root.
func FromPkgInfo(info pkginfo.PkgInfo, root string) error {
	p, err := pkger.ForPkgInfo(info)
	if err != nil {
		return err
	}
	return FromPkgDir(pkger.PkgDir{Pkger: p, Root: root})
}

// skipInstalled reports whether an installed version can stand in while another
// container updates; LockWaitEnv turns the skip off, forcing the wait.
func skipInstalled(pkgDir pkger.PkgDir) func() bool {
	return func() bool {
		if os.Getenv(LockWaitEnv) != "" {
			return false
		}
		active := installutil.CurrentVersion(pkgDir.Current())
		if active == "" {
			return false
		}
		log.Infof("%s update in flight; using installed %s", pkgDir.Name(), active)
		return true
	}
}

// installed reports whether version's dir actually carries the executable: the
// current symlink can outlive its target on the shared volume (a pruned or
// wiped dir, or an install predating the layout), and only the stat catches it.
func installed(pkgDir pkger.PkgDir, version string) bool {
	_, err := os.Stat(filepath.Join(pkgDir.VersionDir(version), pkgDir.RelBin()))
	return err == nil
}

// install puts version in place: download to a temp dir, rename in, then flip
// the current and bin symlinks.
func install(pkgDir pkger.PkgDir, version string) error {
	tmp := pkgDir.TmpDir(version)
	if err := installutil.SafeMkdir(tmp); err != nil {
		return err
	}
	if err := pkgDir.Install(tmp, version); err != nil {
		return fmt.Errorf("install %s %s: %w", pkgDir.Name(), version, err)
	}
	if err := installutil.SafeMv(tmp, pkgDir.VersionDir(version)); err != nil {
		return err
	}
	return updateSymlinks(pkgDir, version)
}

// updateSymlinks flips current and the PATH-exposed bin link to version.
func updateSymlinks(pkgDir pkger.PkgDir, version string) error {
	if err := installutil.ReplaceSymlink(version, pkgDir.Current()); err != nil {
		return err
	}
	return installutil.ReplaceSymlink(pkgDir.BinLinkTarget(), pkgDir.BinLink())
}
