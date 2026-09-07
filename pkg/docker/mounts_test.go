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

func TestVolumeLabels(t *testing.T) {
	assert.Equal(t, map[string]string{"ccbox": "true", "ccbox.project": "/Users/me/proj"}, volumeLabels("/Users/me/proj"))
}

func TestCacheVolumeBinds(t *testing.T) {
	got := cacheVolumeBinds("/Users/me/code/project_name") // host cwd

	// Per-project named volumes keyed on the slug, sorted (deterministic spec).
	assert.Equal(t, []string{
		"ccbox-Users-me-code-project_name-cache:/home/ccbox/.cache",
		"ccbox-Users-me-code-project_name-gem:/home/ccbox/.gem",
		"ccbox-Users-me-code-project_name-go:/home/ccbox/go",
		"ccbox-Users-me-code-project_name-local:/home/ccbox/.local",
		"ccbox-Users-me-code-project_name-npm-global:/home/ccbox/.npm-global",
		"ccbox-Users-me-code-project_name-npm:/home/ccbox/.npm",
		"ccbox-Users-me-code-project_name-tmp:/tmp",
	}, got)
}
