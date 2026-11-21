package artifact

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/tarutil"
)

// TarGz installs a packaged download: the tar.gz carries the whole app tree —
// exec, resources — and extracts into the install dir intact, nothing moved
// piecemeal.
type TarGz struct{ relBin string }

// NewTarGz returns the tar.gz unpacker; relBin is the executable's path inside
// the extracted tree.
func NewTarGz(relBin string) TarGz { return TarGz{relBin: relBin} }

// Install streams the tar.gz into dir, entry by entry.
func (t TarGz) Install(_ context.Context, dir string, targz io.Reader) error {
	gz, err := gzip.NewReader(targz)
	if err != nil {
		return fmt.Errorf("extract tar.gz: %w", err)
	}
	defer gz.Close() //nolint:errcheck // best-effort close of a read-through stream
	if err := tarutil.Extract(dir, gz); err != nil {
		return err
	}
	// the tar's own exec bit can't be trusted to survive packing pipelines
	return os.Chmod(filepath.Join(dir, t.relBin), ExecFileMode) //nolint:gosec // path is the clis root + the firm-validated relBin
}

// RelBin is the executable's path inside the extracted tree.
func (t TarGz) RelBin() string { return t.relBin }
