package install

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/flock"
)

func TestEnvPkgDir_BadEnv(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"unset fails the boot", "", pkginfo.EnvVar},
		{"bad json", `{"name":`, "pkginfo: parse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(pkginfo.EnvVar, tt.body)

			_, err := EnvPkgDir()

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestEnvPkgDir_Root(t *testing.T) {
	tests := []struct {
		name     string
		clisDir  string
		wantRoot string
	}{
		{"default", "", DefaultRoot},
		{"ClisDirEnv override", "/custom/clis", "/custom/clis"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(pkginfo.EnvVar, `{"name":"claude","npm":{"package":"a"}}`)
			t.Setenv(ClisDirEnv, tt.clisDir)

			pkgDir, err := EnvPkgDir()

			require.NoError(t, err)
			assert.Equal(t, "claude", pkgDir.Name())
			assert.Equal(t, tt.wantRoot, pkgDir.Root)
		})
	}
}

// settledPkgDir is a PkgDir the boot's install settled at version.
func settledPkgDir(t *testing.T, version string) pkger.PkgDir {
	t.Helper()
	pkgDir := pkger.PkgDir{Pkger: &fakePkger{name: "claude", latest: version}, Root: clisRoot(t)}
	require.NoError(t, FromPkgDir(t.Context(), pkgDir))
	return pkgDir
}

func TestHold_HoldsLinkedVersion(t *testing.T) {
	pkgDir := settledPkgDir(t, "1.2.3")

	release, err := Hold(pkgDir)
	require.NoError(t, err)

	// the linked version is held for the container's life — prune's probe sees it
	assert.True(t, flock.Held(pkgDir.VersionDir("1.2.3")))
	assert.False(t, flock.Held(pkgDir.VersionDir("2.0.0")), "only the linked version is held")

	require.NoError(t, release())
	assert.False(t, flock.Held(pkgDir.VersionDir("1.2.3")), "released with its holder")
}

func TestHold_MissingBinLink(t *testing.T) {
	// the boot's install always links before the hold: a missing link is a boot
	// failure, not a no-op
	pkgDir := pkger.PkgDir{Pkger: &fakePkger{name: "claude"}, Root: clisRoot(t)}

	_, err := Hold(pkgDir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bin link")
}
