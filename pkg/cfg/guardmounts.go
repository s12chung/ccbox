package cfg

import (
	"slices"
)

var (
	// tmpfsDefaults subbed in for DefaultsAlias
	tmpfsDefaults = []string{".idea", ".vscode", ".zed"}
	// volumeDefaults subbed in for DefaultsAlias
	volumeDefaults = []string{"node_modules", ".venv", "target"}
	// readOnlyDefaults subbed in for DefaultsAlias
	readOnlyDefaults = []string{
		".ccbox.yaml", ".env", "**/*.pem", // the seeded config's inline examples — see defaultsExpansionComment

		".ccbox.local.yaml",
		".env.*", ".envrc",
		"secrets",
		"**/*.key", "**/id_rsa*",
	}
)

// TmpfsDefaults is what DefaultsAlias in tmpfs_masks expands to
func TmpfsDefaults() []string { return slices.Clone(tmpfsDefaults) }

// VolumeDefaults is what DefaultsAlias in volume_masks expands to
func VolumeDefaults() []string { return slices.Clone(volumeDefaults) }

// ReadOnlyDefaults is what DefaultsAlias in read_only_globs expands to
func ReadOnlyDefaults() []string { return slices.Clone(readOnlyDefaults) }
