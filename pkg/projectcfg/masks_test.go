package projectcfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVolumeCleanupDirs(t *testing.T) {
	// all defaults regardless of presence, plus explicit config volumes, deduped
	c := Config{Volumes: []string{"node_modules", "target"}}
	assert.Equal(t, []string{"node_modules", ".venv", "vendor/bundle", "target"}, c.VolumeCleanupDirs())
}
