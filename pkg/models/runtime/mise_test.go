package runtime

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestFromMiseConfig(t *testing.T) {
	t.Run("the picked config's [tools] pins resolve to runtimes", func(t *testing.T) {
		testutil.Home(t)
		projectDir := t.TempDir()
		body := "[tools]\ngo = \"1.26\"\nripgrep = \"15.2\"\n"
		require.NoError(t, os.WriteFile(mise.ProjectConfigPath(projectDir), []byte(body), osutil.File))

		selected, err := FromMiseConfig(projectDir)

		require.NoError(t, err)
		names := make([]string, 0, len(selected))
		for _, rt := range selected {
			names = append(names, rt.Name)
		}
		assert.Equal(t, []string{"go", "node"}, names, "non-runtime pins drop, node stays")
	})

	t.Run("no config to read errors", func(t *testing.T) {
		testutil.Home(t)

		_, err := FromMiseConfig(t.TempDir())

		require.Error(t, err)
	})
}

func TestMiseRuntimes_Packages(t *testing.T) {
	assert.Equal(t, []string{"libatomic1"}, MiseRuntimes(All()).Packages())

	// libs two runtimes share render once — a repeat would redeclare the TOML key
	overlapping := MiseRuntimes{
		{Name: "a", AptPkgs: []string{"libz", "liba"}},
		{Name: "b", AptPkgs: []string{"libz", "libm"}},
	}
	assert.Equal(t, []string{"liba", "libm", "libz"}, overlapping.Packages())
}

func TestMiseRuntimes_CacheVolumes(t *testing.T) {
	assert.Equal(t, AllCacheVolumes(), MiseRuntimes(All()).CacheVolumes())
}
