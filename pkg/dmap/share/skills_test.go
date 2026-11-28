package share

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// The tests cover the skills share's own wiring — the seeded dir and each cli's mount.
// The dir binds as-is: it never goes through scratch.

func TestSafeSeedSkills_SeedsMissing(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, SafeSeedSkills())

	info, err := os.Stat(SkillsDir())
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.Equal(t, osutil.Dir, info.Mode().Perm())
}

func TestSafeSeedSkills_KeepsExisting(t *testing.T) {
	testutil.Home(t)
	require.NoError(t, os.MkdirAll(SkillsDir(), osutil.Dir))
	seeded := filepath.Join(SkillsDir(), "pdf-tools")
	require.NoError(t, os.MkdirAll(seeded, osutil.Dir))

	require.NoError(t, SafeSeedSkills())

	assert.DirExists(t, seeded, "the seeded dir is never touched")
}

func TestSkillsBind(t *testing.T) {
	t.Run("binds the shared dir at the cli's skills mount", func(t *testing.T) {
		testutil.Home(t)
		for _, tc := range []struct {
			cliName string
			mount   string
		}{
			{"claude", ".claude"},
			{"opencode", ".config/opencode"},
			{"pi", ".pi/agent"},
		} {
			assert.Equal(t, klean.ShareBind{
				HostPath:      SkillsDir(),
				ContainerPath: path.Join(pkginfo.ContainerHome, tc.mount, skillsDirName),
			}, SkillsBind(tc.cliName), tc.cliName)
		}
	})

	t.Run("binds over the cli's empty skills dir", func(t *testing.T) {
		testutil.Home(t)
		require.NoError(t, os.MkdirAll(filepath.Join(cli.UserConfigDir("claude"), skillsDirName), osutil.Dir))

		assert.Equal(t, klean.ShareBind{
			HostPath:      SkillsDir(),
			ContainerPath: path.Join(pkginfo.ContainerHome, ".claude", skillsDirName),
		}, SkillsBind("claude"))
	})

	t.Run("no bind when the config home carries its own skills", func(t *testing.T) {
		testutil.Home(t)
		require.NoError(t, os.MkdirAll(filepath.Join(cli.UserConfigDir("claude"), skillsDirName, "pdf-tools"), osutil.Dir))

		assert.Equal(t, klean.ShareBind{}, SkillsBind("claude"))
	})
}
