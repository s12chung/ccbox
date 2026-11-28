package errs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrefix(t *testing.T) {
	err := errors.New("boom")

	require.NoError(t, Prefix("pkginfo", nil))
	wrapped := Prefix("pkginfo", err)
	require.EqualError(t, wrapped, "pkginfo: boom")
	assert.ErrorIs(t, wrapped, err)
}
