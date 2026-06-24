package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAllowOverrideRendersRegex(t *testing.T) {
	got := AllowOverride(false, []string{"example.com", "test.mywebsite.com"})

	body, ok := got[allowFileName]
	require.True(t, ok)
	assert.Equal(t, "(^|\\.)example\\.com$\n(^|\\.)test\\.mywebsite\\.com$\n", string(body))
}

func TestAllowOverrideDefaults(t *testing.T) {
	without := AllowOverride(false, nil)[allowFileName]
	with := AllowOverride(true, []string{"example.com"})[allowFileName]

	assert.Empty(t, without)                                 // defaults off, no domains → empty
	assert.Contains(t, string(with), "(^|\\.)github\\.com$") // a default is present
	assert.Contains(t, string(with), "(^|\\.)example\\.com$")
}

func TestAllowOverrideDoesNotMutateDefaults(t *testing.T) {
	before := len(allowDefaults)
	AllowOverride(true, []string{"example.com"})
	assert.Len(t, allowDefaults, before)
}
