package cmd

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io/fs"
	"maps"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// srcTree is the on-disk ccboxtools tree the check compares against.
var srcTree = fstest.MapFS{
	"ccboxtools/go.mod":     {Data: []byte("module github.com/s12chung/ccbox/ccboxtools\n")},
	"ccboxtools/main.go":    {Data: []byte("package main\n")},
	"ccboxtools/pkg/pkg.go": {Data: []byte("package pkg\n")},
}

// tarFS packs files (name → body) into a context FS holding dist/ccboxtools.tar.gz.
func tarFS(t *testing.T, files map[string][]byte) fs.FS {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, name := range slices.Sorted(maps.Keys(files)) {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Size: int64(len(files[name]))}))
		_, err := tw.Write(files[name])
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return fstest.MapFS{"dist/ccboxtools.tar.gz": &fstest.MapFile{Data: buf.Bytes()}}
}

// freshFiles returns tar bodies matching srcTree exactly.
func freshFiles() map[string][]byte {
	return map[string][]byte{
		"ccboxtools/go.mod":     []byte("module github.com/s12chung/ccbox/ccboxtools\n"),
		"ccboxtools/main.go":    []byte("package main\n"),
		"ccboxtools/pkg/pkg.go": []byte("package pkg\n"),
	}
}

func TestCheckCCBoxtoolsTar(t *testing.T) {
	t.Run("fresh", func(t *testing.T) {
		require.NoError(t, validateToolsTar(tarFS(t, freshFiles()), srcTree))
	})

	t.Run("drifted", func(t *testing.T) {
		files := freshFiles()
		files["ccboxtools/main.go"] = []byte("package main // drifted\n")

		err := validateToolsTar(tarFS(t, files), srcTree)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "run make")
	})

	t.Run("missing from tar", func(t *testing.T) {
		files := freshFiles()
		delete(files, "ccboxtools/pkg/pkg.go")

		err := validateToolsTar(tarFS(t, files), srcTree)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "run make")
	})

	t.Run("stray in tar", func(t *testing.T) {
		files := freshFiles()
		files["stray.txt"] = []byte("junk")

		err := validateToolsTar(tarFS(t, files), srcTree)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "run make")
	})

	t.Run("no go.mod", func(t *testing.T) {
		files := freshFiles()
		delete(files, "ccboxtools/go.mod")

		err := validateToolsTar(tarFS(t, files), srcTree)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "run make")
	})
}
