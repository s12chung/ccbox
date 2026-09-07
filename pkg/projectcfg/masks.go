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

// NotFoundMasks loads the defaulted config (LoadExpanded) and returns its tmpfs and
// volume dirs not found in the project
func NotFoundMasks(projectDir string, flags Config) ([]string, []string, error) {
	c, err := LoadExpanded(projectDir, flags)
	if err != nil {
		return nil, nil, err
	}
	return absentDirs(projectDir, c.Tmpfs), absentDirs(projectDir, c.Volumes), nil
}

func absentDirs(src string, dirs []string) []string {
	present := ioutil.DirsPresentInSrc(src, dirs)
	var out []string
	for _, d := range dirs {
		if !slices.Contains(present, d) {
			out = append(out, d)
		}
	}
	return out
}

// VolumeCleanupDirs is every mask dir whose volume may exist
func (c Config) VolumeCleanupDirs() []string {
	seen := map[string]bool{}
	var dirs []string
	for _, d := range append(append([]string{}, volumeDefaults...), c.Volumes...) {
		if seen[d] {
			continue
		}
		seen[d] = true
		dirs = append(dirs, d)
	}
	return dirs
}
