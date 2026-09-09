package cmd

import (
	"strings"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/pkg/util/uslice"
)

func printPresentGuardMounts() {
	if tmpfsMasks := projectCfg.TmpfsMasksPresent(); len(tmpfsMasks) > 0 {
		log.Infof("during run, masked with temp filesystem: %s", strings.Join(tmpfsMasks, ", "))
	}
	if volumeMasks := projectCfg.VolumeMasksPresent(); len(volumeMasks) > 0 {
		log.Infof("during run, masked with persistent volume: %s", strings.Join(volumeMasks, ", "))
	}
	if roPaths := projectCfg.ReadOnlyPathsPresent(); len(roPaths) > 0 {
		log.Infof("during run, read-only: %s", strings.Join(roPaths, ", "))
	}
}

// warnCreatedGuardMounts warns for each mask dir or glob match the run created: it exists now,
// so future runs guard it — a heads-up that it behaves differently from here.
func warnCreatedGuardMounts(presentMasksBefore, presentPathsBefore []string) {
	nowPresent := append(projectCfg.TmpfsMasksPresent(), projectCfg.VolumeMasksPresent()...)
	if created := uslice.Minus(nowPresent, presentMasksBefore); len(created) > 0 {
		log.Warnf("before run, these directories did not exist. future runs will mask them: %s", strings.Join(created, ", "))
	}
	if created := uslice.Minus(projectCfg.ReadOnlyPathsPresent(), presentPathsBefore); len(created) > 0 {
		log.Warnf("before run, these paths did not exist. future runs will re-mount them read-only: %s", strings.Join(created, ", "))
	}
}
