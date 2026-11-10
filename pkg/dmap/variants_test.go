package dmap

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/util/slug"
)

func TestVariantSuffix(t *testing.T) {
	assert.Empty(t, variantSuffix(false, false), "the proxied headless variant's suffix is empty")
	assert.Equal(t, "-plain", variantSuffix(false, true))
	assert.Equal(t, "-vnc", variantSuffix(true, true))
	assert.Equal(t, "-vnc-proxy", variantSuffix(true, false), "the egress wall makes the proxied vnc combo its own variant")
}

func TestDevboxName(t *testing.T) {
	projectDir := "/Users/me/myproj"
	assert.Equal(t, "ccbox"+slug.Path(projectDir), devboxName(projectDir, false, false), "the proxied headless variant keeps the bare name")
	assert.Equal(t, "ccbox"+slug.Path(projectDir)+"-plain", devboxName(projectDir, false, true))
	assert.Equal(t, "ccbox"+slug.Path(projectDir)+"-vnc", devboxName(projectDir, true, true), "the desktop variant's devbox is its own")
	want := "ccbox" + slug.Path(projectDir) + "-vnc-proxy"
	assert.Equal(t, want, devboxName(projectDir, true, false), "the egress wall makes the proxied vnc combo its own devbox")
}
