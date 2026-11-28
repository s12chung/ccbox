// Package install ensures a CLI's version ask is installed under a clis or
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
	// Version is the run's ask: "latest" tracks the channel, an exact version is pinned
	Version string
}

// FromEnv returns the CLI the run's env names at the clis root, at the config's
// version ask — a boot without either fails.
func FromEnv() (PkgDir, error) {
	pkginfoJSON := os.Getenv(pkginfo.EnvVar)
	if pkginfoJSON == "" {
		return PkgDir{}, fmt.Errorf("%s names no cli", pkginfo.EnvVar)
	}
	info, err := pkginfo.FromJSON(pkginfoJSON)
	if err != nil {
		return PkgDir{}, err
	}
	config, err := pkginfo.RunConfigFromEnv()
	if err != nil {
		return PkgDir{}, err
	}
	pkgDir, err := ForPkgInfo(info, DefaultRoot)
	if err != nil {
		return PkgDir{}, err
	}
	pkgDir.Version = config.CLIVersion
	return pkgDir, nil
}

// ForPkgInfo returns info's CLI or GUI app at root.
func ForPkgInfo(info pkginfo.PkgInfo, root string) (PkgDir, error) {
	p, err := pkger.ForPkgInfo(info)
	if err != nil {
		return PkgDir{}, err
	}
	return PkgDir{Pkger: p, Root: root}, nil
}

// Target is the version the settle lands on — a pinned version never asks the channel.
func (d PkgDir) Target(ctx context.Context) (string, error) {
	if pkginfo.IsPinned(d.Version) {
		return d.Version, nil
	}
	return d.Latest(ctx)
}

// Settle settles the CLI on its target version — installing it when absent,
// pruning the other versions around held ones — and links the container's own
// bin at it, all under the CLI's install lock.
func (d PkgDir) Settle(ctx context.Context) error {
	return flock.Do(ctx, d.LockPath(), func() error {
		target, err := d.Target(ctx)
		if err != nil {
			return fmt.Errorf("resolve %s latest: %w", d.Name(), err)
		}
		if d.isInstalled(target) {
			log.Infof("%s %s is up to date", d.Name(), target)
		} else {
			log.Infof("installing %s %s", d.Name(), target)
			if err := d.installVersion(ctx, target); err != nil {
				return err
			}
			log.Infof("installed %s %s", d.Name(), target)
		}
		if err := installutil.Prune(d.Dir(), target, func(version string) bool {
			// a version another devbox holds for its life is spared
			return flock.Held(d.VersionDir(version))
		}); err != nil {
			return err
		}
		return installutil.ReplaceSymlink(filepath.Join(d.VersionDir(target), d.RelBin()), d.BinLink())
	}, nil)
}

// Hold holds the bin link's version for the container's life, so other
// devboxes' prunes can't delete it under a running session; returns the release.
func (d PkgDir) Hold() (func() error, error) {
	dir, err := d.linkedVersionDir()
	if err != nil {
		return nil, err
	}
	return flock.Hold(dir)
}

// HoldExec holds the bin link's version across syscall.Exec: the exec'd
// session inherits the hold, keeping its version prune-safe for its life.
func (d PkgDir) HoldExec() error {
	dir, err := d.linkedVersionDir()
	if err != nil {
		return err
	}
	return flock.HoldExec(dir)
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

// linkedVersionDir is the version dir the bin link names.
func (d PkgDir) linkedVersionDir() (string, error) {
	version, err := d.LinkedVersion()
	if err != nil {
		return "", err
	}
	return d.VersionDir(version), nil
}
