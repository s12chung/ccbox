package fsutil

import (
	"io"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeDir_ReadDirCursor(t *testing.T) {
	fsys := MustNewFS(fstest.MapFS{
		"a.txt": {Data: []byte("A")},
		"b.txt": {Data: []byte("B")},
		"c.txt": {Data: []byte("C")},
	})

	file, err := fsys.Open(".")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()
	dir, ok := file.(fs.ReadDirFile)
	require.True(t, ok, "dirs are fs.ReadDirFile")

	// Streaming one-by-one ends in EOF.
	var names []string
	for {
		list, err := dir.ReadDir(1)
		for _, e := range list {
			names = append(names, e.Name())
		}
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
	}
	assert.Equal(t, []string{"a.txt", "b.txt", "c.txt"}, names)

	// Exhausted: ReadDir(-1) is empty with a nil error; ReadDir(1) is EOF.
	list, err := dir.ReadDir(-1)
	require.NoError(t, err)
	assert.Empty(t, list)
	_, err = dir.ReadDir(1)
	require.ErrorIs(t, err, io.EOF)

	// A fresh open resets the cursor.
	file, err = fsys.Open(".")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()
	dir, ok = file.(fs.ReadDirFile)
	require.True(t, ok)
	list, err = dir.ReadDir(-1)
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

func TestFakeDir_ReadErrors(t *testing.T) {
	fsys := MustNewFS(fstest.MapFS{"sub/a.txt": {Data: []byte("A")}})

	file, err := fsys.Open("sub")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	info, err := file.Stat()
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "emptied-by-nothing dir still a dir")

	_, err = file.Read(make([]byte, 1))
	require.ErrorContains(t, err, "is a directory")
}

func TestNamedFile_ReportsTreeName(t *testing.T) {
	fsys := MustNewFS(fstest.MapFS{"a.txt": {Data: []byte("A")}})
	require.NoError(t, fsys.Rename("a.txt", "renamed.txt"))

	file, err := fsys.Open("renamed.txt")
	require.NoError(t, err)
	defer func() { require.NoError(t, file.Close()) }()

	info, err := file.Stat()
	require.NoError(t, err)
	assert.Equal(t, "renamed.txt", info.Name())
	assert.False(t, info.IsDir())

	body, err := io.ReadAll(file)
	require.NoError(t, err)
	assert.Equal(t, "A", string(body))
}

func TestDirInfo_Name(t *testing.T) {
	var info fs.FileInfo = dirInfo{name: "d"}
	assert.Equal(t, "d", info.Name())
	assert.True(t, info.IsDir())
	assert.Equal(t, fs.ModeDir|0o555, info.Mode())
	assert.Zero(t, info.Size())
	assert.Zero(t, info.ModTime())
	assert.Nil(t, info.Sys())
}
