package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/s12chung/ccbox/pkg/util/log"
)

// printMasks tells the user which project paths are shadowed, so a hidden path is no surprise.
func printMasks() {
	if len(projectCfg.TmpfsMasks) > 0 {
		log.Infof("during run, masked (ephemeral tmpfs): %s", strings.Join(projectCfg.TmpfsMasks, ", "))
	}
	if len(projectCfg.VolumeMasks) > 0 {
		log.Infof("during run, masked (persistent volume): %s", strings.Join(projectCfg.VolumeMasks, ", "))
	}
}

// masksOnHost returns the mask paths whose existence in the host project cwd matches
// present (mirroring projectcfg's mask present-filter).
func masksOnHost(cwd string, paths []string, present bool) []string {
	var out []string
	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(cwd, p)); (err == nil) == present {
			out = append(out, p)
		}
	}
	return out
}

// warnCreatedMasks warns for each default mask path absent at start that the run created: it
// exists now, so future runs will mask it — a heads-up that it behaves differently from here.
func warnCreatedMasks(cwd string, absentBefore []string) {
	if created := masksOnHost(cwd, absentBefore, true); len(created) > 0 {
		log.Warnf("before run, these paths did not exist. future runs will mask them: %s", strings.Join(created, ", "))
	}
}
