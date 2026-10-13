package pick

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfirmHeadless(t *testing.T) {
	ok, err := Confirm("reseed?")
	require.NoError(t, err)
	assert.False(t, ok)
}
