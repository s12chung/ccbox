package projectcfg

import (
	"slices"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

// MaskDefaults are the built-in dirs masked when present in the project: tmpfs then volume.
// Exposed so callers can spot a run creating one that future runs will start masking.
func MaskDefaults() []string {
	return append(append([]string{}, tmpfsDefaults...), volumeDefaults...)
}

// NotFoundMasks loads the defaulted config (LoadExpanded) and returns its tmpfsMasks and
// volumeMasks paths not found in the project
func NotFoundMasks(projectDir string, flags Config) ([]string, []string, error) {
	c, err := LoadExpanded(projectDir, flags)
	if err != nil {
		return nil, nil, err
	}
	return absentPaths(projectDir, c.TmpfsMasks), absentPaths(projectDir, c.VolumeMasks), nil
}

func absentPaths(src string, paths []string) []string {
	present := ioutil.DirsPresent(src, paths)
	var out []string
	for _, p := range paths {
		if !slices.Contains(present, p) {
			out = append(out, p)
		}
	}
	return out
}

// VolumeCleanupDirs is every mask dir whose volume may exist
func (c Config) VolumeCleanupDirs() []string {
	seen := map[string]bool{}
	var dirs []string
	for _, d := range append(append([]string{}, volumeDefaults...), c.VolumeMasks...) {
		if seen[d] {
			continue
		}
		seen[d] = true
		dirs = append(dirs, d)
	}
	return dirs
}
