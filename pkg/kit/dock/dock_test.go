package dock

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChownMnt(t *testing.T) {
	assert.Equal(t, "/mnt/ccbox-vol", chownMnt(OwnedVolume{Name: "ccbox-vol"}))
}

func TestChownBind(t *testing.T) {
	assert.Equal(t, "ccbox-vol:/mnt/ccbox-vol", chownBind(OwnedVolume{Name: "ccbox-vol"}))
}
