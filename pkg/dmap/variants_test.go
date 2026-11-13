package dmap

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/util/slug"
)

func TestVariantSuffix(t *testing.T) {
	assert.Empty(t, variantSuffix(false, true), "the proxied headless variant's suffix is empty")
	assert.Equal(t, "-plain", variantSuffix(false, false), "-plain marks the unproxied ask")
	assert.Equal(t, "-plain-vnc", variantSuffix(true, false))
	assert.Equal(t, "-vnc", variantSuffix(true, true))
}

func TestDevboxName(t *testing.T) {
	projectDir := "/Users/me/myproj"
	assert.Equal(t, "ccbox"+slug.Path(projectDir), devboxName(projectDir, false, true), "the proxied headless variant keeps the bare name")
	assert.Equal(t, "ccbox"+slug.Path(projectDir)+"-plain", devboxName(projectDir, false, false))
	assert.Equal(t, "ccbox"+slug.Path(projectDir)+"-plain-vnc", devboxName(projectDir, true, false), "the desktop variant's devbox is its own")
	assert.Equal(t, "ccbox"+slug.Path(projectDir)+"-vnc", devboxName(projectDir, true, true), "the egress wall makes proxied vnc its own devbox")
}
