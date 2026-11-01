package pkger

import (
	"context"
	"io"
)

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
