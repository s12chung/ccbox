// Package pkger resolves a coding CLI's latest version from its release channel
// and installs that version into a directory.
package pkger

import (
	"context"
	"fmt"
	"io"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger/artifact"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger/dler"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger/npm"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

// Pkger resolves a CLI's latest version and installs it, per its install source.
type Pkger interface {
	// Name is the CLI's name; the installed executable is expected to match it.
	Name() string
	// Latest returns the release channel's latest version.
	Latest(ctx context.Context) (string, error)
	// Install downloads the version into dir.
	Install(ctx context.Context, dir, version string) error
	// RelBin is the installed executable's path relative to the install dir.
	RelBin() string
}

// ForPkgInfo returns info's install source as a Pkger.
func ForPkgInfo(info pkginfo.PkgInfo) (Pkger, error) {
	switch {
	case info.Npm != nil:
		return npm.New(info.Name, *info.Npm), nil
	default: // info is validated: exactly one of Npm/ReleaseURL is set
		d, err := pkgerize(*info.ReleaseURL)
		if err != nil {
			return nil, err
		}
		installer, err := newInstaller(*info.ReleaseURL, info.Name)
		if err != nil {
			return nil, err
		}
		return FilePkger{name: info.Name, Downloader: d, Installer: installer}, nil
	}
}

// pkgerize returns the download source for the pkginfo.ReleaseURL, its url's
// $arch resolved for the running arch
func pkgerize(rel pkginfo.ReleaseURL) (Downloader, error) {
	if rel.DownloadTemplate == nil {
		return dler.NewJQ(rel.ArchedURL(), rel.JQSchema)
	}
	return dler.NewURLTemplate(rel.ArchedURL(), *rel.DownloadTemplate), nil
}

func newInstaller(rel pkginfo.ReleaseURL, name string) (Installer, error) {
	if rel.Artifact == nil {
		return artifact.NewRawBin(name), nil
	}
	switch rel.Artifact.Type {
	case "deb":
		return artifact.NewDeb(rel.Artifact.RelBin), nil
	case "targz":
		return artifact.NewTarGz(rel.Artifact.RelBin), nil
	default: // unrepresentable via firm-validated config; Go-constructed configs hit it
		return nil, fmt.Errorf("pkger: unknown artifact type %q", rel.Artifact.Type)
	}
}

// Downloader resolves a release channel's latest version and fetches a
// version's file for the running arch.
type Downloader interface {
	// Latest returns the release channel's latest version.
	Latest(ctx context.Context) (string, error)
	// Download fetches version's file for the running arch; the caller closes it.
	Download(ctx context.Context, version string) (io.ReadCloser, error)
}

// Installer places a downloaded executable into an install dir.
type Installer interface {
	// Install writes the downloaded executable into dir, executable.
	Install(ctx context.Context, dir string, exe io.Reader) error
	// RelBin is the installed executable's path relative to the install dir.
	RelBin() string
}

// FilePkger installs a CLI that is one downloaded file: the Downloader
// fetches the version's file, the Installer puts it in place.
type FilePkger struct {
	Downloader
	Installer

	name string
}

// Name is the CLI's name.
func (p FilePkger) Name() string { return p.name }

// Latest returns the release channel's latest version.
func (p FilePkger) Latest(ctx context.Context) (string, error) { return p.Downloader.Latest(ctx) }

// Install downloads the version's file and places it into dir.
func (p FilePkger) Install(ctx context.Context, dir, version string) error {
	exe, err := p.Download(ctx, version)
	if err != nil {
		return err
	}
	defer exe.Close() //nolint:errcheck // failing is ok
	return p.Installer.Install(ctx, dir, exe)
}

// RelBin is the installed executable's path relative to the install dir.
func (p FilePkger) RelBin() string { return p.Installer.RelBin() }
