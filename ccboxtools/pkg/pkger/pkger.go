// Package pkger resolves a coding CLI's latest version from its release channel
// and installs that version into a directory.
package pkger

import (
	"path/filepath"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

// Pkger resolves a CLI's latest version and installs it, per its install source.
type Pkger interface {
	// Name is the CLI's name; the installed executable is expected to match it.
	Name() string
	// Latest returns the release channel's latest version.
	Latest() (string, error)
	// Install downloads the version into dir.
	Install(dir, version string) error
	// RelBin is the installed executable's path relative to the install dir.
	RelBin() string
}

// For returns info's install source as a Pkger.
func For(info pkginfo.PkgInfo) Pkger {
	switch {
	case info.Npm != nil:
		return Npm{Npm: *info.Npm, name: info.Name}
	default: // info is validated: exactly one of Npm/VersionURL is set
		return VersionURL{VersionURL: *info.VersionURL, name: info.Name}
	}
}

// PkgDir is a Pkger bound to a clis root: the volume paths the CLI installs to.
type PkgDir struct {
	Pkger

	Root string
}

// Dir is the CLI's dir under the root, holding every installed version.
func (d PkgDir) Dir() string { return filepath.Join(d.Root, d.Name()) }

// VersionDir is a version's install dir.
func (d PkgDir) VersionDir(version string) string { return filepath.Join(d.Dir(), version) }

// TmpDir is a version's staging dir, renamed onto VersionDir once installed.
func (d PkgDir) TmpDir(version string) string { return filepath.Join(d.Dir(), ".tmp-"+version) }

// Current is the symlink pointing at the active VersionDir.
func (d PkgDir) Current() string { return filepath.Join(d.Dir(), "current") }

// BinLink is the PATH-exposed symlink pointing at the active executable.
func (d PkgDir) BinLink() string { return filepath.Join(d.Root, "bin", d.Name()) }

// BinLinkTarget is BinLink's target, relative to the bin dir.
func (d PkgDir) BinLinkTarget() string {
	return filepath.Join("..", d.Name(), "current", d.RelBin())
}

// LockPath is the per-CLI lock file guarding installs. The flock on it is
// kernel-held — released when the holding container dies — so the persisted
// file needs no stale handling.
func (d PkgDir) LockPath() string { return filepath.Join(d.Root, d.Name()+".lock") }
