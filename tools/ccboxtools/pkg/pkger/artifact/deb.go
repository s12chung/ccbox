package artifact

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// Deb installs a packaged download: the deb carries the whole app tree — exec,
// resources, libs — and extracts into the install dir intact, nothing moved
// piecemeal.
type Deb struct{ relBin string }

// NewDeb returns the deb unpacker; relBin is the executable's path inside the
// extracted tree.
func NewDeb(relBin string) Deb { return Deb{relBin: relBin} }

// Install copies the deb to a dot-prefixed temp under dir, dpkg-deb-extracts
// the tree into it, and removes the temp.
func (d Deb) Install(dir string, deb io.Reader) error {
	// dot-prefixed temp: SafeMv renames dir itself onto the version path, so the temp
	// must not leak into the extracted tree's listing
	path := filepath.Join(dir, "."+filepath.Base(d.relBin)+".deb")
	//nolint:gosec // path is the staging dir's own temp file
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer os.Remove(path) //nolint:errcheck // best-effort cleanup of the temp
	if _, err := io.Copy(file, deb); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := file.Close(); err != nil {
		return err
	}

	if out, err := extractDebCmd(path, dir).CombinedOutput(); err != nil {
		return fmt.Errorf("extract deb: %w: %s", err, out)
	}
	return nil
}

func extractDebCmd(path, dir string) *exec.Cmd {
	//nolint:gosec // fixed argv on a deb from the firm-validated URL
	return exec.CommandContext(context.Background(), "dpkg-deb", "-x", path, dir)
}

// RelBin is the executable's path inside the extracted tree.
func (d Deb) RelBin() string { return d.relBin }
