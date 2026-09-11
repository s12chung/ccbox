package slug

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPath(t *testing.T) {
	assert.Equal(t, "-Users-me-app", Path("/Users/me/app"))
	assert.Equal(t, "name", Path("name"))
	assert.Equal(t, "-", Path("/"))
	assert.Equal(t, "-a-b--c", Path("/a/b-c"))
	assert.Equal(t, "a--b", Path("a-b"))
	assert.Equal(t, "a--b-c--d", Path("a-b/c-d"))
}

func TestPath_Injective(t *testing.T) {
	assert.NotEqual(t, Path("/a/b-c"), Path("/a/b/c"))
}
