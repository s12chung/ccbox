package projectcfg

import (
	"slices"
)

var (
	// tmpfsDefaults subbed in for DefaultsToken
	tmpfsDefaults = []string{".idea", ".vscode"}
	// volumeDefaults subbed in for DefaultsToken
	volumeDefaults = []string{"node_modules", ".venv", "vendor/bundle"}
	// readOnlyDefaults subbed in for DefaultsToken
	readOnlyDefaults = []string{
		".ccbox.yaml", ".ccbox.local.yaml",
		".env", ".env.*", ".envrc",
		"secrets",
		"**/*.pem", "**/*.key",
	}
)

// TmpfsDefaults is what DefaultsToken in tmpfs_masks expands to
func TmpfsDefaults() []string { return slices.Clone(tmpfsDefaults) }

// VolumeDefaults is what DefaultsToken in volume_masks expands to
func VolumeDefaults() []string { return slices.Clone(volumeDefaults) }

// ReadOnlyDefaults is what DefaultsToken in read_only_globs expands to
func ReadOnlyDefaults() []string { return slices.Clone(readOnlyDefaults) }
