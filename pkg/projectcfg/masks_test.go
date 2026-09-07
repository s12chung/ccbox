package projectcfg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVolumeCleanupDirs(t *testing.T) {
	c := Config{VolumeMasks: []string{"node_modules", "target"}}
	assert.Equal(t, []string{"node_modules", ".venv", "vendor/bundle", "target"}, c.VolumeCleanupDirs())
}
