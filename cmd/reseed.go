package cmd

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/log"
	"github.com/s12chung/ccbox/pkg/prompt"
	"github.com/s12chung/ccbox/pkg/seed"
)

var reseedCmd = &cobra.Command{
	Use:   "reseed",
	Short: "Seed the host config dir for the configured CLI from the embedded seed, backing up overwrites",
	RunE: func(_ *cobra.Command, _ []string) error {
		_, err := safeSeedConfig(flagCacheDir, projectCfg.CLI, true)
		return err
	},
}

// safeSeedConfig seeds cli's host config dir in cacheDir and returns that dir:
// the shared all/ tree first (renamed to cli's live memory file), then cli's own tree.
func safeSeedConfig(cacheDir string, cliName harness.Name, confirm bool) (string, error) {
	cli := harness.MustFor(cliName)
	name := string(cli.Name)
	return safeSeed(seedFS, filepath.Join(cacheDir, name), confirm,
		seedSrc{
			sub:     harness.SharedSeedPath,
			renames: map[string]string{harness.AgentsFileName: cli.SeedAgentsFilename},
		},
		seedSrc{sub: name},
	)
}

// seedTreeFn is seed.Tree, indirected so tests can stub out the file-copying step.
var seedTreeFn = seed.Tree

// seedSrc is one embedded tree to lay onto the dest dir.
type seedSrc struct {
	sub     string            // subtree under seed.SourceDir
	renames map[string]string // source path -> destination name
}

// safeSeed seeds dst from root's embedded trees under seed.SourceDir when dst
// doesn't exist yet, applying renames — or re-seeds after confirmation when confirm
// is set, backing up overwritten files across all trees.
func safeSeed(root fs.FS, dst string, confirm bool, srcs ...seedSrc) (string, error) {
	switch _, err := os.Stat(dst); {
	case err == nil: // exists
		if !confirm {
			return dst, nil
		}
		if !prompt.Confirm(dst + " exists; reseed and back up overwritten files?") {
			log.Info("reseed aborted")
			return dst, nil
		}
	case !os.IsNotExist(err): // stat failed for some other reason
		return dst, err
	}

	var renamed []string
	for _, s := range srcs {
		src, err := fs.Sub(root, path.Join(seed.SourceDir, s.sub))
		if err != nil {
			return dst, err
		}
		r, err := seedTreeFn(src, dst, s.renames)
		if err != nil {
			return dst, err
		}
		renamed = append(renamed, r...)
	}
	if len(renamed) == 0 {
		log.Infof("seeded: %s with no overwritten files", dst)
	} else {
		rel := make([]string, len(renamed))
		for i, p := range renamed {
			rel[i], _ = filepath.Rel(dst, p)
		}
		log.Infof("seeded %s, backed up overwritten files:\n%s", dst, strings.Join(rel, "\n"))
	}
	return dst, nil
}
