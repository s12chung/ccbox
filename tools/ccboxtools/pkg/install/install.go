// Package install ensures a CLI's latest version is installed under the clis
// root: resolve it, install if absent, link the container's own bin at it,
// prune old versions around held ones.
package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/flock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/installutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// DefaultRoot is the clis root's container mount: the ccbox-clis global volume's.
const DefaultRoot = "/opt/ccbox/clis"

// AppsRoot is the apps root's container mount: the ccbox-apps global volume's,
// where the GUI app (the VNC_CONFIG env's gui_app) installs at start.
const AppsRoot = "/opt/ccbox/apps"

// FromPkgDir settles pkgDir on its latest version — installing it when absent,
// pruning the other versions around held ones — and links the container's own
// bin at it, all under the CLI's install lock.
func FromPkgDir(ctx context.Context, pkgDir pkger.PkgDir) error {
	return flock.Do(ctx, pkgDir.LockPath(), func() error {
		latest, err := pkgDir.Latest(ctx)
		if err != nil {
			return fmt.Errorf("resolve %s latest: %w", pkgDir.Name(), err)
		}
		if installed(pkgDir, latest) {
			log.Infof("%s %s is up to date", pkgDir.Name(), latest)
		} else {
			log.Infof("installing %s %s", pkgDir.Name(), latest)
			if err := install(ctx, pkgDir, latest); err != nil {
				return err
			}
			log.Infof("installed %s %s", pkgDir.Name(), latest)
		}
		if err := installutil.Prune(pkgDir.Dir(), latest, func(version string) bool {
			// a version another devbox holds for its life is spared
			return flock.Held(pkgDir.VersionDir(version))
		}); err != nil {
			return err
		}
		return installutil.ReplaceSymlink(filepath.Join(pkgDir.VersionDir(latest), pkgDir.RelBin()), pkgDir.BinLink())
	}, nil)
}

// FromPkgInfo installs info's CLI or GUI app at root.
func FromPkgInfo(ctx context.Context, info pkginfo.PkgInfo, root string) error {
	p, err := pkger.ForPkgInfo(info)
	if err != nil {
		return err
	}
	return FromPkgDir(ctx, pkger.PkgDir{Pkger: p, Root: root})
}

// installed reports whether version's dir actually carries the executable — a
// dir on the shared volume can lack it, and only the stat catches that.
func installed(pkgDir pkger.PkgDir, version string) bool {
	_, err := os.Stat(filepath.Join(pkgDir.VersionDir(version), pkgDir.RelBin()))
	return err == nil
}

func install(ctx context.Context, pkgDir pkger.PkgDir, version string) error {
	tmp := pkgDir.TmpDir(version)
	if err := installutil.SafeMkdir(tmp); err != nil {
		return err
	}
	if err := pkgDir.Install(ctx, tmp, version); err != nil {
		return fmt.Errorf("install %s %s: %w", pkgDir.Name(), version, err)
	}
	return installutil.SafeMv(tmp, pkgDir.VersionDir(version))
}
