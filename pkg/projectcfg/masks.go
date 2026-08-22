package projectcfg

// MaskDefaults are the built-in dirs masked when present in the workspace: tmpfs then volume.
// Exposed so callers can spot a run creating one that future runs will start masking.
func MaskDefaults() []string {
	return append(append([]string{}, tmpfsDefaults...), volumeDefaults...)
}

// VolumeCleanupDirs is every mask dir whose volume may exist: the built-in defaults (regardless of
// presence) plus explicit config volumes.
func (c Config) VolumeCleanupDirs() []string {
	seen := map[string]bool{}
	var dirs []string
	for _, d := range append(append([]string{}, volumeDefaults...), c.Volumes...) {
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	return dirs
}
