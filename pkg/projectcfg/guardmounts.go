package projectcfg

import (
	"slices"
)

var (
	// tmpfsDefaults subbed in for DefaultsAlias
	tmpfsDefaults = []string{".idea", ".vscode"}
	// volumeDefaults subbed in for DefaultsAlias
	volumeDefaults = []string{"node_modules", ".venv", "vendor/bundle"}
	// readOnlyDefaults subbed in for DefaultsAlias
	readOnlyDefaults = []string{
		".ccbox.yaml", ".ccbox.local.yaml",
		".env", ".env.*", ".envrc",
		"secrets",
		"**/*.pem", "**/*.key",
	}
)

// TmpfsDefaults is what DefaultsAlias in tmpfs_masks expands to
func TmpfsDefaults() []string { return slices.Clone(tmpfsDefaults) }

// VolumeDefaults is what DefaultsAlias in volume_masks expands to
func VolumeDefaults() []string { return slices.Clone(volumeDefaults) }

// ReadOnlyDefaults is what DefaultsAlias in read_only_globs expands to
func ReadOnlyDefaults() []string { return slices.Clone(readOnlyDefaults) }
