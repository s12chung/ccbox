package install

import (
	"fmt"
	"os"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

// ClisDirEnv overrides the clis root; the image mounts the global volume at the default.
const ClisDirEnv = "CCBOX_CLIS_DIR"

// LockWaitEnv forces waiting on a held install lock when set, instead of
// skipping to the installed version.
const LockWaitEnv = "CCBOX_LOCK_WAIT"

// FromEnv updates the CLI the container env describes: pkginfo.EnvVar's JSON picks
// the CLI, ClisDirEnv overrides the clis root. The env-driven entry both the entrypoint
// and `ccboxtools update` go through.
func FromEnv() error {
	pkginfoJSON := os.Getenv(pkginfo.EnvVar)
	if pkginfoJSON == "" {
		return fmt.Errorf("%s is not set", pkginfo.EnvVar)
	}
	info, err := pkginfo.FromJSON(pkginfoJSON)
	if err != nil {
		return err
	}
	root := os.Getenv(ClisDirEnv)
	if root == "" {
		root = DefaultRoot
	}
	return FromPkgInfo(info, root)
}
