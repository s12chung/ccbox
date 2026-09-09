package toolsbuild

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoRoot resolves this file's location up to the repo root, since go test
// runs with the package dir as cwd
func repoRoot(t *testing.T) string {
	t.Helper()

	_, this, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Dir(filepath.Dir(filepath.Dir(this))) // pkg/toolsbuild → repo root
}

func TestBuildCmd(t *testing.T) {
	cmd := buildCmd(context.Background(), "arm64", "/tmp/roots/out/ccboxtools")

	assert.Equal(t, []string{
		"go", "build", "-trimpath", "-buildvcs=false", "-o", "/tmp/roots/out/ccboxtools", ".",
	}, cmd.Args)
	assert.Equal(t, "ccboxtools", cmd.Dir)
	assert.Equal(t, append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH=arm64"), cmd.Env)
}

func TestBuild(t *testing.T) {
	t.Chdir(repoRoot(t))

	out := filepath.Join(t.TempDir(), "ccboxtools")
	require.NoError(t, Build(context.Background(), runtime.GOARCH, out))

	info, err := os.Stat(out)
	require.NoError(t, err)
	assert.Positive(t, info.Size())
}

func TestBuild_CompileError(t *testing.T) {
	t.Chdir(repoRoot(t))

	// badarch
	err := Build(context.Background(), "badarch", filepath.Join(t.TempDir(), "ccboxtools"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "go build ccboxtools")
}
