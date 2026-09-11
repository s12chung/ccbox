package slug

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPath(t *testing.T) {
	assert.Equal(t, "-Users-me-app", Path("/Users/me/app"))
	assert.Equal(t, "name", Path("name"))
	assert.Equal(t, "-", Path("/"))
}
