package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/util/prompt"
)

func TestTinyproxyLevelColor(t *testing.T) {
	assert.Equal(t, prompt.ColorGreen,
		tinyproxyLevelColor("NOTICE   Jun 22 21:45:16.558 [1]: Reloading"))
	assert.Equal(t, prompt.ColorDim,
		tinyproxyLevelColor("INFO     Jun 22 21:45:16.558 [1]: listening"))

	assert.Empty(t, tinyproxyLevelColor("DEBUG    something"), "unknown level uncolored")
	assert.Empty(t, tinyproxyLevelColor("   "), "blank line uncolored")
}
