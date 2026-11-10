package install

import (
	"context"
	"os"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

// ClisDirEnv overrides the clis root; the image mounts the global volume at the default.
const ClisDirEnv = "CCBOX_CLIS_DIR"

// LockWaitEnv forces waiting on a held install lock when set, instead of
// skipping to the installed version.
const LockWaitEnv = "CCBOX_LOCK_WAIT"

// FromEnv installs the CLI the run's env names — no-op when it names none.
// pkginfo.EnvVar's JSON picks the CLI, ClisDirEnv overrides the clis root.
func FromEnv(ctx context.Context) error {
	pkginfoJSON := os.Getenv(pkginfo.EnvVar)
	if pkginfoJSON == "" {
		return nil
	}
	info, err := pkginfo.FromJSON(pkginfoJSON)
	if err != nil {
		return err
	}
	root := os.Getenv(ClisDirEnv)
	if root == "" {
		root = DefaultRoot
	}
	return FromPkgInfo(ctx, info, root)
}
