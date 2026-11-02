package dmap

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/util/slug"
)

func TestVariantSuffix(t *testing.T) {
	assert.Empty(t, variantSuffix(false, false), "the proxied headless boot's suffix is empty")
	assert.Equal(t, plainSuffix, variantSuffix(false, true))
	assert.Equal(t, vncSuffix, variantSuffix(true, true))
}

func TestDevboxName(t *testing.T) {
	projectDir := "/Users/me/myproj"
	assert.Equal(t, "ccbox"+slug.Path(projectDir), devboxName(projectDir, false, false), "the proxied headless boot keeps the bare name")
	assert.Equal(t, "ccbox"+slug.Path(projectDir)+plainSuffix, devboxName(projectDir, false, true))
	assert.Equal(t, "ccbox"+slug.Path(projectDir)+vncSuffix, devboxName(projectDir, true, true), "the desktop variant's devbox is its own")
}
