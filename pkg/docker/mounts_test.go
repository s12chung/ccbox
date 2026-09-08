package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeContainerPath(t *testing.T) {
	got, err := safeContainerPath("/home/ccbox/proj", "vendor/bundle")
	require.NoError(t, err)
	assert.Equal(t, "/home/ccbox/proj/vendor/bundle", got)

	for _, p := range []string{"..", "../escape", "../../x"} {
		_, err := safeContainerPath("/home/ccbox/proj", p)
		assert.Error(t, err, p)
	}
}
