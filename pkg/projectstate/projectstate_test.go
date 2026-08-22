package projectstate

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func TestSeedFS(t *testing.T) {
	f := SeedFS()
	assert.NoError(t, fstest.TestFS(f, "README.md"))
}
