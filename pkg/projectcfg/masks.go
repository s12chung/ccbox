package projectcfg

import (
	"slices"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

var (
	// tmpfsDefaults always-masked paths, prepended only when present in the project (to prevent host creation)
	tmpfsDefaults = []string{".idea", ".vscode"}
	// volumeDefaults for persistent volume masked paths only when present in the project (to prevent host creation)
	volumeDefaults = []string{"node_modules", ".venv", "vendor/bundle"}
)

// TmpfsDefaults is what DefaultsToken in tmpfsMasks expands to
func TmpfsDefaults() []string { return slices.Clone(tmpfsDefaults) }

// VolumeDefaults is what DefaultsToken in volumeMasks expands to
func VolumeDefaults() []string { return slices.Clone(volumeDefaults) }

// MaskDefaults are the built-in paths masked when present in the project: tmpfs then volume.
func MaskDefaults() []string { return append(TmpfsDefaults(), VolumeDefaults()...) }

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
	present := ioutil.PathsPresent(src, paths)
	var out []string
	for _, p := range paths {
		if !slices.Contains(present, p) {
			out = append(out, p)
		}
	}
	return out
}
