package cmd

import (
	"strings"

	"github.com/s12chung/ccbox/pkg/util/slicex"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

func printPresentGuardMounts() {
	if tmpfsMasks := config.TmpfsMasksPresent(); len(tmpfsMasks) > 0 {
		log.Infof("during run, masked with temp filesystem: %s", strings.Join(tmpfsMasks, ", "))
	}
	if volumeMasks := config.VolumeMasksPresent(); len(volumeMasks) > 0 {
		log.Infof("during run, masked with persistent volume: %s", strings.Join(volumeMasks, ", "))
	}
	if roPaths := config.ReadOnlyPathsPresent(); len(roPaths) > 0 {
		log.Infof("during run, read-only: %s", strings.Join(roPaths, ", "))
	}
}

// warnCreatedGuardMounts warns for each mask dir or glob match the run created: it exists now,
// so future runs guard it — a heads-up that it behaves differently from here.
func warnCreatedGuardMounts(presentMasksBefore, presentPathsBefore []string) {
	nowPresent := append(config.TmpfsMasksPresent(), config.VolumeMasksPresent()...)
	if created := slicex.Minus(nowPresent, presentMasksBefore); len(created) > 0 {
		log.Warnf("before run, these directories did not exist. future runs will mask them: %s", strings.Join(created, ", "))
	}
	if created := slicex.Minus(config.ReadOnlyPathsPresent(), presentPathsBefore); len(created) > 0 {
		log.Warnf("before run, these paths did not exist. future runs will re-mount them read-only: %s", strings.Join(created, ", "))
	}
}
