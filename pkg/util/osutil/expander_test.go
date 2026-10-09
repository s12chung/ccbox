package osutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpander_IsStale(t *testing.T) {
	const source = "[tools]\n"

	// expand the source, as an Ensure leaves it behind
	expand := func(t *testing.T, e Expander) {
		t.Helper()
		require.NoError(t, os.WriteFile(e.Source, []byte(source), File))
		require.NoError(t, e.Expand([]byte("expansion")))
	}

	t.Run("missing expansion is stale", func(t *testing.T) {
		e := testExpander(t)
		require.NoError(t, os.WriteFile(e.Source, []byte(source), File))
		stale, err := e.IsStale()
		require.NoError(t, err)
		assert.True(t, stale)
	})

	t.Run("missing sha256 is stale", func(t *testing.T) {
		e := testExpander(t)
		require.NoError(t, os.WriteFile(e.Source, []byte(source), File))
		require.NoError(t, SafeWriteFile(e.Expansion, []byte("expansion")))
		stale, err := e.IsStale()
		require.NoError(t, err)
		assert.True(t, stale)
	})

	t.Run("changed source is stale", func(t *testing.T) {
		e := testExpander(t)
		expand(t, e)

		require.NoError(t, os.WriteFile(e.Source, []byte(source+"node = \"26\"\n"), File))
		stale, err := e.IsStale()
		require.NoError(t, err)
		assert.True(t, stale)
	})

	t.Run("unchanged source reuses the expansion", func(t *testing.T) {
		e := testExpander(t)
		expand(t, e)

		stale, err := e.IsStale()
		require.NoError(t, err)
		assert.False(t, stale)
	})

	t.Run("hand-committed sha256 reuses the expansion", func(t *testing.T) {
		e := testExpander(t)
		expand(t, e)

		sum, err := e.sha256()
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(e.SHA256Path(), []byte(sum+"\n"), File))
		stale, err := e.IsStale()
		require.NoError(t, err)
		assert.False(t, stale)
	})
}

func TestExpander_Expand(t *testing.T) {
	e := testExpander(t)
	require.NoError(t, os.WriteFile(e.Source, []byte("[tools]\n"), File))

	require.NoError(t, e.Expand([]byte("expansion")))

	body, err := os.ReadFile(e.Expansion) // #nosec G304 -- the test's own expansion path
	require.NoError(t, err)
	assert.Equal(t, "expansion", string(body))
	stale, err := e.IsStale()
	require.NoError(t, err)
	assert.False(t, stale)
}

// testExpander builds an Expander over fresh paths
func testExpander(t *testing.T) Expander {
	t.Helper()
	dir := t.TempDir()
	return Expander{
		Source:    filepath.Join(dir, "source.toml"),
		Expansion: filepath.Join(dir, "expansion.lock"),
	}
}
