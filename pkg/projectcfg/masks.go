package projectcfg

import (
	"slices"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

var (
	// tmpfsDefaults always-masked dirs, prepended only when present in the project (to prevent host creation)
	tmpfsDefaults = []string{".idea", ".vscode"}
	// volumeDefaults for persistent volume masked dirs only when present in the project (to prevent host creation)
	volumeDefaults = []string{"node_modules", ".venv", "vendor/bundle"}
)

// TmpfsDefaults is what DefaultsToken in tmpfsMasks expands to
func TmpfsDefaults() []string { return slices.Clone(tmpfsDefaults) }

// VolumeDefaults is what DefaultsToken in volumeMasks expands to
func VolumeDefaults() []string { return slices.Clone(volumeDefaults) }

// MaskDefaults are the built-in dirs masked when present in the project: tmpfs then volume.
func MaskDefaults() []string { return append(TmpfsDefaults(), VolumeDefaults()...) }

// NotFoundMasks loads the defaulted config (LoadExpanded) and returns its tmpfsMasks and
// volumeMasks dirs not found in the project
func NotFoundMasks(projectDir string, flags Config) ([]string, []string, error) {
	c, err := LoadExpanded(projectDir, flags)
	if err != nil {
		return nil, nil, err
	}
	return absentDirs(projectDir, c.TmpfsMasks), absentDirs(projectDir, c.VolumeMasks), nil
}

func absentDirs(src string, dirs []string) []string {
	present := ioutil.DirsPresent(src, dirs)
	var out []string
	for _, d := range dirs {
		if !slices.Contains(present, d) {
			out = append(out, d)
		}
	}
	return out
}
