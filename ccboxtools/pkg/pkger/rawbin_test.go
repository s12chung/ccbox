package pkger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRawBin_Install(t *testing.T) {
	b := RawBin{name: "grok"}

	dir := t.TempDir()
	require.NoError(t, b.Install(dir, strings.NewReader("#!/bin/sh\n")))

	// the raw binary lands at the dir root, executable
	//nolint:gosec // test fixture path
	body, err := os.ReadFile(filepath.Join(dir, "grok"))
	require.NoError(t, err)
	assert.Equal(t, "#!/bin/sh\n", string(body))
	info, err := os.Stat(filepath.Join(dir, "grok"))
	require.NoError(t, err)
	assert.Equal(t, execFileMode, info.Mode().Perm())

	assert.Equal(t, "grok", b.RelBin())

	t.Run("missing dir", func(t *testing.T) {
		err := b.Install(filepath.Join(dir, "missing"), strings.NewReader("#!/bin/sh\n"))
		assert.Error(t, err)
	})
}
