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

// TmpfsMasksExpanded expands the DefaultsToken tokens in TmpfsMasks to defaults
func (c *Config) TmpfsMasksExpanded() []string {
	if c.expandedTmpfsMasks == nil {
		c.expandedTmpfsMasks = expandList(c.TmpfsMasks, tmpfsDefaults)
	}
	return c.expandedTmpfsMasks
}

// TmpfsMasksPresent filters TmpfsMasksExpanded() to the paths present as dirs in the project
func (c *Config) TmpfsMasksPresent() []string {
	if c.presentTmpfsMasks == nil {
		c.presentTmpfsMasks = ioutil.DirsPresent(c.projectDir, c.TmpfsMasksExpanded())
	}
	return c.presentTmpfsMasks
}

// TmpfsMasksAbsent filters TmpfsMasksExpanded() to the paths NOT present as dirs in the project
func (c *Config) TmpfsMasksAbsent() []string {
	return absentDirs(c.projectDir, c.TmpfsMasksExpanded())
}

// VolumeMasksExpanded expands the DefaultsToken tokens in VolumeMasks to defaults
func (c *Config) VolumeMasksExpanded() []string {
	if c.expandedVolumeMasks == nil {
		c.expandedVolumeMasks = expandList(c.VolumeMasks, volumeDefaults)
	}
	return c.expandedVolumeMasks
}

// VolumeMasksPresent filters VolumeMasksExpanded() to the paths present as dirs in the project
func (c *Config) VolumeMasksPresent() []string {
	if c.presentVolumeMasks == nil {
		c.presentVolumeMasks = ioutil.DirsPresent(c.projectDir, c.VolumeMasksExpanded())
	}
	return c.presentVolumeMasks
}

// VolumeMasksAbsent filters VolumeMasksExpanded() to the paths NOT present as dirs in the project
func (c *Config) VolumeMasksAbsent() []string {
	return absentDirs(c.projectDir, c.VolumeMasksExpanded())
}

// VolumeCleanupDirs is every mask dir whose volume may exist
func (c *Config) VolumeCleanupDirs() []string {
	seen := map[string]bool{}
	var dirs []string
	for _, d := range append(append([]string{}, volumeDefaults...), c.VolumeMasksExpanded()...) {
		if seen[d] {
			continue
		}
		seen[d] = true
		dirs = append(dirs, d)
	}
	return dirs
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
