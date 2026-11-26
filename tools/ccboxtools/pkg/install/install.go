// Package install ensures a CLI's latest version is installed under a clis or
// apps root: resolve it, install if absent, link the container's own bin at it,
// prune old versions around held ones.
package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/flock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/installutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

const (
	// DefaultRoot is the clis root's container mount: the ccbox-clis global volume's.
	DefaultRoot = "/opt/ccbox/clis"
	// AppsRoot is the apps root's container mount: the ccbox-apps global volume's,
	// where the GUI app (the VNC_CONFIG env's gui_app) installs at start.
	AppsRoot = "/opt/ccbox/apps"
)

// PkgDir is a Pkger bound to a clis or apps root: the volume paths the CLI
// installs to, and the install the boot settles, then holds for the container's life.
type PkgDir struct {
	pkger.Pkger

	Root string
}

// FromEnv returns the CLI the run's env names at the clis root — a boot
// without one fails.
func FromEnv() (PkgDir, error) {
	pkginfoJSON := os.Getenv(pkginfo.EnvVar)
	if pkginfoJSON == "" {
		return PkgDir{}, fmt.Errorf("%s names no cli", pkginfo.EnvVar)
	}
	info, err := pkginfo.FromJSON(pkginfoJSON)
	if err != nil {
		return PkgDir{}, err
	}
	return ForPkgInfo(info, DefaultRoot)
}

// ForPkgInfo returns info's CLI or GUI app at root.
func ForPkgInfo(info pkginfo.PkgInfo, root string) (PkgDir, error) {
	p, err := pkger.ForPkgInfo(info)
	if err != nil {
		return PkgDir{}, err
	}
	return PkgDir{Pkger: p, Root: root}, nil
}

// Settle settles the CLI on its latest version — installing it when absent,
// pruning the other versions around held ones — and links the container's own
// bin at it, all under the CLI's install lock.
func (d PkgDir) Settle(ctx context.Context) error {
	return flock.Do(ctx, d.LockPath(), func() error {
		latest, err := d.Latest(ctx)
		if err != nil {
			return fmt.Errorf("resolve %s latest: %w", d.Name(), err)
		}
		if d.isInstalled(latest) {
			log.Infof("%s %s is up to date", d.Name(), latest)
		} else {
			log.Infof("installing %s %s", d.Name(), latest)
			if err := d.installVersion(ctx, latest); err != nil {
				return err
			}
			log.Infof("installed %s %s", d.Name(), latest)
		}
		if err := installutil.Prune(d.Dir(), latest, func(version string) bool {
			// a version another devbox holds for its life is spared
			return flock.Held(d.VersionDir(version))
		}); err != nil {
			return err
		}
		return installutil.ReplaceSymlink(filepath.Join(d.VersionDir(latest), d.RelBin()), d.BinLink())
	}, nil)
}

// Hold holds the bin link's version for the container's life, so other
// devboxes' prunes can't delete it under a running session; returns the release.
func (d PkgDir) Hold() (func() error, error) {
	version, err := d.LinkedVersion()
	if err != nil {
		return nil, err
	}
	return flock.Hold(d.VersionDir(version))
}

// Dir is the CLI's dir under the root, holding every installed version.
func (d PkgDir) Dir() string { return filepath.Join(d.Root, d.Name()) }

// VersionDir is a version's install dir.
func (d PkgDir) VersionDir(version string) string { return filepath.Join(d.Dir(), version) }

// TmpDir is a version's staging dir, renamed onto VersionDir once installed.
func (d PkgDir) TmpDir(version string) string { return filepath.Join(d.Dir(), ".tmp-"+version) }

// BinLink is the PATH-exposed symlink naming the active executable, in the
// container's own bin dir — never the volume's.
func (d PkgDir) BinLink() string { return filepath.Join(filepath.Dir(d.Root), "bin", d.Name()) }

// LinkedVersion is the version BinLink names — whatever the boot's install wrote.
func (d PkgDir) LinkedVersion() (string, error) {
	dest, err := os.Readlink(d.BinLink())
	if err != nil {
		return "", fmt.Errorf("read %s's bin link: %w", d.Name(), err)
	}
	return filepath.Base(strings.TrimSuffix(dest, d.RelBin())), nil
}

// LockPath is the per-CLI install lock — at the root, out of prune's reach.
func (d PkgDir) LockPath() string { return filepath.Join(d.Root, d.Name()+".lock") }

// isInstalled reports whether version's dir actually carries the executable — a
// dir on the shared volume can lack it, and only the stat catches that.
func (d PkgDir) isInstalled(version string) bool {
	_, err := os.Stat(filepath.Join(d.VersionDir(version), d.RelBin()))
	return err == nil
}

// installVersion stages version in its tmp dir, then moves it into place.
func (d PkgDir) installVersion(ctx context.Context, version string) error {
	tmp := d.TmpDir(version)
	if err := installutil.SafeMkdir(tmp); err != nil {
		return err
	}
	if err := d.Install(ctx, tmp, version); err != nil {
		return fmt.Errorf("install %s %s: %w", d.Name(), version, err)
	}
	return installutil.SafeMv(tmp, d.VersionDir(version))
}
