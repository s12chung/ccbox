package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildEnvBaseWins(t *testing.T) {
	base := []string{"http_proxy=wall", "GH_TOKEN=secret"}
	extra := map[string]string{"GOFLAGS": "-mod=mod", "http_proxy": "evil"}

	got := buildEnv(base, extra)

	// extra is sorted and precedes base, so base's http_proxy is the last (winning) value.
	assert.Equal(t, []string{"GOFLAGS=-mod=mod", "http_proxy=evil", "http_proxy=wall", "GH_TOKEN=secret"}, got)
}

func TestBuildTmpfsMasksPaths(t *testing.T) {
	got, err := buildTmpfs("/home/ccbox/proj", []string{".idea", "dist"})
	require.NoError(t, err)

	assert.Equal(t, map[string]string{
		"/home/ccbox/proj/.idea": tmpfsOpts,
		"/home/ccbox/proj/dist":  tmpfsOpts,
	}, got)
}

func TestBuildTmpfsRejectsEscape(t *testing.T) {
	for _, p := range []string{"..", "../escape", "../../x"} {
		_, err := buildTmpfs("/home/ccbox/proj", []string{p})
		assert.Error(t, err, p)
	}
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
	}, got)
}
