package install

import (
	"fmt"
	"os"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkger"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

// ClisDirEnv overrides the clis root; the image mounts the global volume at the default.
const ClisDirEnv = "CCBOX_CLIS_DIR"

// RunFromEnv updates the CLI the container env describes: pkginfo.EnvVar's JSON picks
// the CLI, ClisDirEnv overrides the clis root. The env-driven entry both the entrypoint
// and `ccboxtools update` go through.
func RunFromEnv() error {
	body := os.Getenv(pkginfo.EnvVar)
	if body == "" {
		return fmt.Errorf("%s is not set", pkginfo.EnvVar)
	}
	info, err := pkginfo.FromJSON(body)
	if err != nil {
		return err
	}

	root := os.Getenv(ClisDirEnv)
	if root == "" {
		root = DefaultRoot
	}
	return Run(pkger.PkgDir{Pkger: pkger.For(info), Root: root})
}
