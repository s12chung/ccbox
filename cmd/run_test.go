package cmd

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

const testProjectDir = "/work/myproj"

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

func TestSafeSeedProjectStateDir_MissingSeeds(t *testing.T) {
	userDir := t.TempDir()
	wantDir := projectStateDir(userDir, testProjectDir)

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
	require.NoError(t, os.MkdirAll(projectStateDir(userDir, testProjectDir), ioutil.Dir))

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
