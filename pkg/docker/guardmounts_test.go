package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTmpfsMasks(t *testing.T) {
	got, err := tmpfsMasks("/Users/me/proj", []string{".idea", "dist"})
	require.NoError(t, err)

	assert.Equal(t, map[string]string{
		"/home/ccbox/proj/.idea": tmpfsOpts,
		"/home/ccbox/proj/dist":  tmpfsOpts,
	}, got)
}

func TestNamedVolumeMasks(t *testing.T) {
	binds, names, err := namedVolumeMasks("/Users/me/proj", []string{"node_modules", "vendor/bundle"})
	require.NoError(t, err)

	// bind is "volume:containerPath"; the name slugifies the path (/ → -) under the project slug.
	assert.Equal(t, []string{
		"ccbox-Users-me-proj-node_modules:/home/ccbox/proj/node_modules",
		"ccbox-Users-me-proj-vendor-bundle:/home/ccbox/proj/vendor/bundle",
	}, binds)
	assert.Equal(t, []string{"ccbox-Users-me-proj-node_modules", "ccbox-Users-me-proj-vendor-bundle"}, names)
}

func TestReadOnlyPathBinds(t *testing.T) {
	got, err := readOnlyPathBinds("/Users/me/proj", []string{".env", "certs/server.pem"})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"/Users/me/proj/.env:/home/ccbox/proj/.env:ro",
		"/Users/me/proj/certs/server.pem:/home/ccbox/proj/certs/server.pem:ro",
	}, got)

	for _, path := range []string{"../escape", "../../x"} {
		_, err := readOnlyPathBinds("/Users/me/proj", []string{path})
		assert.Error(t, err, path)
	}
}
