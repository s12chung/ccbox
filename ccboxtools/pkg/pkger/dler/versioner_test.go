package dler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTextVersioner(t *testing.T) {
	got, err := TextVersioner{}.Latest([]byte("  1.0.5\n"))
	require.NoError(t, err)
	assert.Equal(t, "1.0.5", got)
}
