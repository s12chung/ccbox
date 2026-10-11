package prov

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/slicex"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// useHome points HOME at a fresh temp dir
func useHome(t *testing.T) {
	t.Helper()
	testutil.Home(t)
}

// writeSpec writes body to path
func writeSpec(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), osutil.Dir))
	require.NoError(t, os.WriteFile(path, []byte(body), osutil.File))
}

func TestLoad_MergesLayers(t *testing.T) {
	tests := []struct {
		name          string
		user, project string
		want          Spec
	}{
		{"empty layers select nothing", "", "", Spec{}},
		{"user layer alone", "runtimes: [go]\n", "", Spec{Runtimes: []string{"go"}}},
		{
			"project layer alone", "", "runtimes: [go, ruby]\napt_packages: [htop]\n",
			Spec{Runtimes: []string{"go", "ruby"}, AptPackages: []string{"htop"}},
		},
		{
			"lists append across layers", "runtimes: [go]\napt_packages: [htop]\n", "runtimes: [python]\napt_packages: [jq]\n",
			Spec{Runtimes: []string{"go", "python"}, AptPackages: []string{"htop", "jq"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useHome(t)
			if tt.user != "" {
				writeSpec(t, userConfigFile(), tt.user)
			}
			projectDir := t.TempDir()
			if tt.project != "" {
				writeSpec(t, projectConfigFile(projectDir), tt.project)
			}

			provSpec, err := Load(projectDir)
			require.NoError(t, err)
			assert.Equal(t, tt.want, *provSpec)
		})
	}
}

func TestLoad_Validation(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"node is not selectable", "runtimes: [node]\n", "is not one of"},
		{"unknown runtime errors", "runtimes: [rust]\n", "is not one of"},
		{"bad apt package errors", "runtimes: [go]\napt_packages: [Htop]\n", "does not match"},
		{"unknown field errors", "runtimes: [go]\nbogus: 1\n", "field bogus not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			useHome(t)
			writeSpec(t, userConfigFile(), tt.body)

			_, err := Load(t.TempDir())
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestResolve_All corroborates a full selection against the runtime table.
func TestResolve_All(t *testing.T) {
	provSpec := Spec{Runtimes: selectableRuntimes()}
	resolved := provSpec.Resolve()

	assert.Equal(t, runtime.All(), resolved.Runtimes)
	assert.Equal(t, runtime.AllMiseTools(), resolved.MiseTools())
	assert.Equal(t, runtime.AllCacheVolumes(), resolved.CacheVolumes())
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name            string
		runtimes        []string
		aptPackages     []string
		wantNames       []string
		wantAptPackages []string
	}{
		{"node rides every resolution", nil, nil, []string{"node"}, nil},
		{"resolved runtimes come in canonical order", []string{"ruby"}, nil, []string{"node", "ruby"}, nil},
		{"unknown names skip", []string{"bogus"}, nil, []string{"node"}, nil},
		{"apt package repeats drop", []string{"go"}, []string{"htop", "jq", "htop"}, []string{"go", "node"}, []string{"htop", "jq"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provSpec := Spec{Runtimes: tt.runtimes, AptPackages: tt.aptPackages}
			resolved := provSpec.Resolve()

			assert.Equal(t, tt.wantNames, slicex.Map(resolved.Runtimes, func(r runtime.Runtime) string { return r.Name }))
			assert.Equal(t, tt.wantAptPackages, resolved.AptPackages)
		})
	}
}

func TestSeedConfig(t *testing.T) {
	useHome(t)

	require.NoError(t, SeedConfig())

	// the seed is the pinned defaults' carrier
	body, err := os.ReadFile(userConfigFile()) // #nosec G304 -- the test's own seeded path
	require.NoError(t, err)
	golden, err := os.ReadFile(filepath.Join("testdata", "TestSeedConfig.yaml")) // #nosec G304 -- the test's own fixture
	require.NoError(t, err)
	assert.Equal(t, string(golden), string(body))
}

func TestSeedConfig_SkipsExisting(t *testing.T) {
	useHome(t)
	writeSpec(t, userConfigFile(), "runtimes: [go]\n")

	require.NoError(t, SeedConfig())

	body, err := os.ReadFile(userConfigFile()) // #nosec G304 -- the test's own seeded path
	require.NoError(t, err)
	assert.Equal(t, "runtimes: [go]\n", string(body), "the user's own file is never touched")
}

// TestSeedConfig_Loads pins the seed round-tripping through Load.
func TestSeedConfig_Loads(t *testing.T) {
	useHome(t)
	require.NoError(t, SeedConfig())

	provSpec, err := Load(t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, selectableRuntimes(), provSpec.Runtimes)
	assert.Empty(t, provSpec.AptPackages)
}
