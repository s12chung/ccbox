package cmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/dmap"
	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

const testProjectDir = "/work/myproj"

func TestResumeArgs(t *testing.T) {
	defer func() { runModes.Resume = false }()

	tests := []struct {
		name    string
		resume  bool
		args    []string
		wantErr bool
	}{
		{name: "no args is fine", resume: false, args: nil},
		{name: "name with -r is fine", resume: true, args: []string{"auth-refactor"}},
		{name: "name without -r is rejected", resume: false, args: []string{"auth-refactor"}, wantErr: true},
		{name: "two names is rejected", resume: true, args: []string{"a", "b"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runModes.Resume = tt.resume
			err := resumeArgs(nil, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSafeSeedProjectStateDir_MissingSeeds(t *testing.T) {
	userDir := t.TempDir()
	wantDir := dmap.ProjectStateDir(userDir, testProjectDir)

	var gotDir string
	called := false
	defer stubSeedTreeFn(func(_ fs.FS, dest string) ([]string, error) {
		called, gotDir = true, dest
		return nil, nil
	})()

	require.NoError(t, safeSeedProjectStateDir(userDir, testProjectDir))
	assert.True(t, called, "seedTreeFn not called for missing dir")
	assert.Equal(t, wantDir, gotDir, "seeded dir")
}

func TestSafeSeedProjectStateDir_ExistingSkips(t *testing.T) {
	userDir := t.TempDir()
	require.NoError(t, os.MkdirAll(dmap.ProjectStateDir(userDir, testProjectDir), ioutil.Dir))

	called := false
	defer stubSeedTreeFn(func(fs.FS, string) ([]string, error) {
		called = true
		return nil, nil
	})()

	require.NoError(t, safeSeedProjectStateDir(userDir, testProjectDir))
	assert.False(t, called, "seedTreeFn called for existing dir")
}

func TestSafeSeedProjectStateDir_PropagatesSeedError(t *testing.T) {
	wantErr := errors.New("boom")
	defer stubSeedTreeFn(func(fs.FS, string) ([]string, error) {
		return nil, wantErr
	})()

	assert.ErrorIs(t, safeSeedProjectStateDir(t.TempDir(), testProjectDir), wantErr)
}

func TestSafeSeedCLIDataBinds(t *testing.T) {
	t.Run("creates file at slugged host path", func(t *testing.T) {
		userDir := t.TempDir()
		content := "{}"
		cli := harness.CLI{Name: "opencode", DataBinds: map[string]*string{".local/share/opencode/auth.json": &content}}

		require.NoError(t, safeSeedCLIDataBinds(userDir, cli))

		got, err := os.ReadFile(
			dmap.CLIDataBindPath(userDir, "opencode", ".local/share/opencode/auth.json"),
		) // #nosec G304 -- the test's own seeded path
		require.NoError(t, err)
		assert.Equal(t, content, string(got))
	})

	t.Run("creates dir", func(t *testing.T) {
		userDir := t.TempDir()
		cli := harness.CLI{Name: "mycli", DataBinds: map[string]*string{".local/share/mycli/store": nil}}

		require.NoError(t, safeSeedCLIDataBinds(userDir, cli))

		info, err := os.Stat(dmap.CLIDataBindPath(userDir, "mycli", ".local/share/mycli/store"))
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("skips existing file", func(t *testing.T) {
		userDir := t.TempDir()
		content := "{}"
		cli := harness.CLI{Name: "opencode", DataBinds: map[string]*string{".local/share/opencode/auth.json": &content}}
		host := dmap.CLIDataBindPath(userDir, "opencode", ".local/share/opencode/auth.json")

		require.NoError(t, os.MkdirAll(filepath.Dir(host), ioutil.Dir))
		require.NoError(t, os.WriteFile(host, []byte(`{"real":"creds"}`), ioutil.File))

		require.NoError(t, safeSeedCLIDataBinds(userDir, cli))

		got, err := os.ReadFile(host) // #nosec G304 -- the test's own seeded path
		require.NoError(t, err)
		assert.JSONEq(t, `{"real":"creds"}`, string(got), "existing file — real creds — never clobbered")
	})
}
