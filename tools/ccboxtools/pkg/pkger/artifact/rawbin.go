package artifact

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// ExecFileMode is the mode installed executables carry (mirrors the host's ioutil.ExecFile).
const ExecFileMode os.FileMode = 0o755

// RawBin installs a downloaded raw executable at the install dir's root.
type RawBin struct{ name string }

// NewRawBin returns the raw placement; name is the executable's name.
func NewRawBin(name string) RawBin { return RawBin{name: name} }

// Install writes the downloaded executable into dir at name, executable.
func (b RawBin) Install(dir string, exe io.Reader) error {
	path := filepath.Join(dir, b.name)
	//nolint:gosec // path is the clis root + the firm-validated CLI name
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, ExecFileMode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, exe); err != nil {
		return errors.Join(err, file.Close())
	}
	return errors.Join(file.Chmod(ExecFileMode), file.Close())
}

// RelBin is the raw binary dropped at the install dir's root.
func (b RawBin) RelBin() string { return b.name }
