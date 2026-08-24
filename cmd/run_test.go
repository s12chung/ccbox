package cmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

const testWorkspace = "/work/myproj"

func TestResumeArgs(t *testing.T) {
	defer func() { flagResume = false }()

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
			flagResume = tt.resume
			err := resumeArgs(nil, tt.args)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSafeSeedProjectDirMissingSeeds(t *testing.T) {
	userDir := t.TempDir()
	wantDir := projectDir(userDir, testWorkspace)

	var gotDir string
	called := false
	defer stubSeedTreeFn(func(_ fs.FS, dest string) ([]string, error) {
		called, gotDir = true, dest
		return nil, nil
	})()

	require.NoError(t, safeSeedProjectDir(userDir, testWorkspace))
	assert.True(t, called, "seedTreeFn not called for missing dir")
	assert.Equal(t, wantDir, gotDir, "seeded dir")
}

func TestSafeSeedProjectDirExistingSkips(t *testing.T) {
	userDir := t.TempDir()
	require.NoError(t, os.MkdirAll(projectDir(userDir, testWorkspace), ioutil.Dir))

	called := false
	defer stubSeedTreeFn(func(fs.FS, string) ([]string, error) {
		called = true
		return nil, nil
	})()

	require.NoError(t, safeSeedProjectDir(userDir, testWorkspace))
	assert.False(t, called, "seedTreeFn called for existing dir")
}

func TestSafeSeedProjectDirPropagatesSeedError(t *testing.T) {
	wantErr := errors.New("boom")
	defer stubSeedTreeFn(func(fs.FS, string) ([]string, error) {
		return nil, wantErr
	})()

	assert.ErrorIs(t, safeSeedProjectDir(t.TempDir(), testWorkspace), wantErr)
}

func TestMasksOnHost(t *testing.T) {
	cwd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(cwd, "vendor", "bundle"), ioutil.Dir))
	require.NoError(t, os.Mkdir(filepath.Join(cwd, "node_modules"), ioutil.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(cwd, "afile"), nil, ioutil.File)) // a file, not a dir

	dirs := []string{"node_modules", "dist", "vendor/bundle", "typo", "afile"}
	assert.Equal(t, []string{"dist", "typo", "afile"}, masksOnHost(cwd, dirs, false))        // absent as a dir (file counts as absent)
	assert.Equal(t, []string{"node_modules", "vendor/bundle"}, masksOnHost(cwd, dirs, true)) // present as a dir
}
