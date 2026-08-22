package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/s12chung/ccbox/pkg/util/log"
)

// printMasks tells the user which workspace dirs are shadowed, so a hidden dir is no surprise.
func printMasks() {
	if len(projectCfg.Tmpfs) > 0 {
		log.Infof("during run, masked (ephemeral tmpfs): %s", strings.Join(projectCfg.Tmpfs, ", "))
	}
	if len(projectCfg.Volumes) > 0 {
		log.Infof("during run, masked (persistent volume): %s", strings.Join(projectCfg.Volumes, ", "))
	}
}

// masksOnHost returns the mask dirs whose existence as a dir in the host workspace cwd matches
// present (dir-only, mirroring projectcfg's mask present-filter, so a stray file never counts).
func masksOnHost(cwd string, dirs []string, present bool) []string {
	var out []string
	for _, d := range dirs {
		info, err := os.Stat(filepath.Join(cwd, d))
		if (err == nil && info.IsDir()) == present {
			out = append(out, d)
		}
	}
	return out
}

// warnCreatedMasks warns for each default mask dir absent at start that the run created: it
// exists now, so future runs will mask it — a heads-up that it behaves differently from here.
func warnCreatedMasks(cwd string, absentBefore []string) {
	if created := masksOnHost(cwd, absentBefore, true); len(created) > 0 {
		log.Warnf("before run, these directories did not exist. future runs will mask them: %s", strings.Join(created, ", "))
	}
}
