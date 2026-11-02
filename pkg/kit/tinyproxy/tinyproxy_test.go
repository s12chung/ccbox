package tinyproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/util/term"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

func TestLevelColor(t *testing.T) {
	assert.Equal(t, term.ColorGreen,
		LevelColor(pkginfo.ProxyMuxName+": NOTICE   Jun 22 21:45:16.558 [1]: Reloading"))
	assert.Equal(t, term.ColorDim,
		LevelColor(pkginfo.ProxyMuxName+": INFO     Jun 22 21:45:16.558 [1]: listening"))

	assert.Empty(t, LevelColor("boot: installing"), "the boot's own lines uncolored")
	assert.Empty(t, LevelColor(pkginfo.ProxyMuxName+": DEBUG    something"), "unknown level uncolored")
	assert.Empty(t, LevelColor(pkginfo.ProxyMuxName+":"), "a tagless-body line uncolored")
	assert.Empty(t, LevelColor("   "), "blank line uncolored")
}
