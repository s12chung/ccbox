package fsutil

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// globFS is a flat tree of <name>.yaml entries, plus a stray file
func globFS() fstest.MapFS {
	return fstest.MapFS{
		"a.yaml": {Data: []byte("body-a")},
		"b.yaml": {Data: []byte("body-b")},
		"c.txt":  {Data: []byte("stray")},
	}
}

// globLoad stands in for a tree's load: each <name>.yaml entry is named after
// its file; the body "bad" fails loading, anything else is not an entry
func globLoad(p string, info EntryInfo) (string, error) {
	if !strings.HasSuffix(p, ".yaml") {
		return "", Skip
	}
	name := strings.TrimSuffix(p, ".yaml")
	if string(info.Body) == "bad" {
		return "", fmt.Errorf("%s: bad body", name)
	}
	return name + "=" + string(info.Body), nil
}

func TestLoadGlob_Loads(t *testing.T) {
	loaded, warns, err := LoadGlob(globFS(), "*", globLoad)

	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Equal(t, []string{"a=body-a", "b=body-b"}, loaded) // glob order; c.txt is skipped
}

func TestLoadGlob_WarnsEntry(t *testing.T) {
	fsys := globFS()
	fsys["b.yaml"] = &fstest.MapFile{Data: []byte("bad")}

	loaded, warns, err := LoadGlob(fsys, "*", globLoad)

	require.NoError(t, err)
	assert.Equal(t, []string{"a=body-a"}, loaded)
	require.Len(t, warns, 1)
	assert.ErrorContains(t, warns[0], "b: bad body")
}

func TestLoadGlob_NoMatches(t *testing.T) {
	loaded, warns, err := LoadGlob(globFS(), "*.md", globLoad)

	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Empty(t, loaded)
}
