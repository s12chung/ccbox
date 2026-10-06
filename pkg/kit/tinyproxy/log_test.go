package tinyproxy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/util/prompt"
)

func TestLevelColor(t *testing.T) {
	assert.Equal(t, prompt.ColorGreen,
		levelColor("NOTICE   Jun 22 21:45:16.558 [1]: Reloading"))
	assert.Equal(t, prompt.ColorDim,
		levelColor("INFO     Jun 22 21:45:16.558 [1]: listening"))

	assert.Empty(t, levelColor("DEBUG    something"), "unknown level uncolored")
	assert.Empty(t, levelColor("   "), "blank line uncolored")
}
