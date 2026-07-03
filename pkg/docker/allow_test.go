package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowOverrideRendersRegex(t *testing.T) {
	got := AllowOverride([]string{"example.com", "test.mywebsite.com"})

	body, ok := got[allowFileName]
	require.True(t, ok)
	assert.Equal(t, "(^|\\.)example\\.com$\n(^|\\.)test\\.mywebsite\\.com$\n", string(body))
}
