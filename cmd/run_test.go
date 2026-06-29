package cmd

import (
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/perm"
)

const testWorkspace = "/work/myproj"

// stubSeedProjectFn swaps the project seed step for a test double and returns a restore func.
func stubSeedProjectFn(fn func(fs.FS, string) ([]string, error)) func() {
	orig := seedProjectFn
	seedProjectFn = fn
	return func() { seedProjectFn = orig }
}

func TestContainerCmd(t *testing.T) {
	defer func() { flagContinue, flagResume, flagShell = false, false, false }()

	tests := []struct {
		name                string
		cont, resume, shell bool
		args                []string
		want                []string
	}{
		{name: "default launches claude", want: []string{"claude"}},
		{name: "continue resumes the last session", cont: true, want: []string{"claude", "-c"}},
		{name: "bare resume opens the session picker", resume: true, want: []string{"claude", "--resume"}},
		{name: "named resume targets that session", resume: true, args: []string{"auth-refactor"}, want: []string{"claude", "--resume", "auth-refactor"}},
		{name: "shell falls back to the image default", shell: true, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flagContinue, flagResume, flagShell = tt.cont, tt.resume, tt.shell
			assert.Equal(t, tt.want, containerCmd(tt.args))
		})
	}
}

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
	cacheDir := t.TempDir()
	wantDir := projectDir(cacheDir, testWorkspace)

	var gotDir string
	called := false
	defer stubSeedProjectFn(func(_ fs.FS, dest string) ([]string, error) {
		called, gotDir = true, dest
		return nil, nil
	})()

	dir, err := safeSeedProjectDir(cacheDir, testWorkspace)
	require.NoError(t, err)
	assert.True(t, called, "seedProjectFn not called for missing dir")
	assert.Equal(t, wantDir, gotDir, "seeded dir")
	assert.Equal(t, wantDir, dir)
}

func TestSafeSeedProjectDirExistingSkips(t *testing.T) {
	cacheDir := t.TempDir()
	wantDir := projectDir(cacheDir, testWorkspace)
	require.NoError(t, os.MkdirAll(wantDir, perm.Dir))

	called := false
	defer stubSeedProjectFn(func(fs.FS, string) ([]string, error) {
		called = true
		return nil, nil
	})()

	dir, err := safeSeedProjectDir(cacheDir, testWorkspace)
	require.NoError(t, err)
	assert.False(t, called, "seedProjectFn called for existing dir")
	assert.Equal(t, wantDir, dir)
}

func TestSafeSeedProjectDirPropagatesSeedError(t *testing.T) {
	wantErr := errors.New("boom")
	defer stubSeedProjectFn(func(fs.FS, string) ([]string, error) {
		return nil, wantErr
	})()

	_, err := safeSeedProjectDir(t.TempDir(), testWorkspace)
	assert.ErrorIs(t, err, wantErr)
}
