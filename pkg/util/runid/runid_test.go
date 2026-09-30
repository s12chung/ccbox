package runid

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	id := New()
	require.Regexp(t, `^[0-9a-f]{8}$`, id)

	seen := map[string]bool{}
	for range 100 {
		seen[New()] = true
	}
	assert.NotContains(t, seen, id, "a fresh id is not drawn again")
}
