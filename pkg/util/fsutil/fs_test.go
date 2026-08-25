package fsutil

import (
	"errors"
	"io/fs"
	"path"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFS(t *testing.T) {
	a := fstest.MapFS{
		"a.txt":      {Data: []byte("A")},
		"shared.txt": {Data: []byte("from A")},
	}
	b := fstest.MapFS{
		"b.txt":          {Data: []byte("B")},
		"shared.txt":     {Data: []byte("from B")}, // overrides a's
		"sub/deep/c.txt": {Data: []byte("C")},
	}

	fsys := MustNewFS(a).MustMerge(b)

	require.NoError(t, fstest.TestFS(fsys, "a.txt", "b.txt", "shared.txt", "sub/deep/c.txt"))

	body, err := fs.ReadFile(fsys, "sub/deep/c.txt")
	require.NoError(t, err)
	assert.Equal(t, "C", string(body))

	body, err = fs.ReadFile(fsys, "shared.txt")
	require.NoError(t, err)
	assert.Equal(t, "from B", string(body), "later merges own colliding paths")
}

func TestRename(t *testing.T) {
	fsys := MustNewFS(fstest.MapFS{
		"a.txt":     {Data: []byte("A")},
		"sub/b.txt": {Data: []byte("B")},
	})
	require.NoError(t, fsys.Rename("a.txt", "renamed.txt"))
	require.NoError(t, fsys.Rename("sub/b.txt", "deep/c.txt"))

	require.NoError(t, fstest.TestFS(fsys, "renamed.txt", "deep/c.txt"))

	// Sources are gone; contents land at the destinations under their new names.
	for _, p := range []string{"a.txt", "sub/b.txt"} {
		_, err := fs.Stat(fsys, p)
		require.ErrorIsf(t, err, fs.ErrNotExist, p)
	}
	for p, want := range map[string]string{"renamed.txt": "A", "deep/c.txt": "B"} {
		info, err := fsys.Stat(p)
		require.NoErrorf(t, err, p)
		assert.Equalf(t, path.Base(p), info.Name(), "tree name reported: %s", p)
		body, err := fs.ReadFile(fsys, p)
		require.NoErrorf(t, err, p)
		assert.Equalf(t, want, string(body), p)
	}

	var got []string
	require.NoError(t, fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		if !d.IsDir() {
			got = append(got, p)
		}
		return nil
	}))
	assert.ElementsMatch(t, []string{"renamed.txt", "deep/c.txt"}, got)

	// Chained renames keep serving the original owner path.
	require.NoError(t, fsys.Rename("renamed.txt", "final.md"))
	body, err := fs.ReadFile(fsys, "final.md")
	require.NoError(t, err)
	assert.Equal(t, "A", string(body))
}

func TestRenameDir(t *testing.T) {
	fsys := MustNewFS(fstest.MapFS{"sub/b.txt": {Data: []byte("B")}})
	require.NoError(t, fsys.Rename("sub", "other"))

	require.NoError(t, fstest.TestFS(fsys, "other/b.txt"))
	body, err := fs.ReadFile(fsys, "other/b.txt")
	require.NoError(t, err)
	assert.Equal(t, "B", string(body), "children keep serving their origins")
}

func TestRenameOverwritesDestination(t *testing.T) {
	fsys := MustNewFS(fstest.MapFS{
		"my_dir/original.txt":    {Data: []byte("ORIGINAL")},
		"my_dir/replacement.txt": {Data: []byte("REPLACEMENT")},
	})
	require.NoError(t, fsys.Rename("my_dir/original.txt", "my_dir/replacement.txt"))

	require.NoError(t, fstest.TestFS(fsys, "my_dir/replacement.txt"))

	_, err := fs.Stat(fsys, "my_dir/original.txt")
	require.ErrorIs(t, err, fs.ErrNotExist)
	body, err := fs.ReadFile(fsys, "my_dir/replacement.txt")
	require.NoError(t, err)
	assert.Equal(t, "ORIGINAL", string(body), "destination replaced by the moved file")

	var got []string
	require.NoError(t, fs.WalkDir(fsys, ".", func(p string, _ fs.DirEntry, err error) error {
		require.NoError(t, err)
		got = append(got, p)
		return nil
	}))
	assert.Equal(t, []string{".", "my_dir", "my_dir/replacement.txt"}, got)
}

func TestRenameSourceMissing(t *testing.T) {
	fsys := MustNewFS(fstest.MapFS{"a.txt": {Data: []byte("A")}})

	err := fsys.Rename("missing.txt", "x.txt")
	require.ErrorContains(t, err, `rename source "missing.txt" not found`)
}

func TestRenameDestinationInvalid(t *testing.T) {
	fsys := MustNewFS(fstest.MapFS{"a.txt": {Data: []byte("A")}})

	for _, dest := range []string{"../evil", ""} {
		err := fsys.Rename("a.txt", dest)
		require.ErrorContainsf(t, err, "invalid rename destination", dest)
	}
	err := fsys.Rename(".", "x.txt")
	require.ErrorContains(t, err, `cannot rename "."`)
}

func TestWalkDir(t *testing.T) {
	tests := []struct {
		name string
		fsys *FS
		want []string
	}{
		{
			name: "nested single",
			fsys: MustNewFS(fstest.MapFS{
				"z.txt":     {Data: []byte("Z")},
				"top/a.txt": {Data: []byte("A")},
			}),
			want: []string{".", "top", "top/a.txt", "z.txt"},
		},
		{
			name: "merged override lists once",
			fsys: MustNewFS(fstest.MapFS{"s/a.txt": {Data: []byte("1")}}).
				MustMerge(fstest.MapFS{
					"s/a.txt": {Data: []byte("2")},
					"t/b.txt": {Data: []byte("B")},
				}),
			want: []string{".", "s", "s/a.txt", "t", "t/b.txt"},
		},
		{
			name: "renames move paths",
			fsys: mustRenamedFS(),
			want: []string{".", "deep", "deep/c.txt", "renamed.txt", "sub"}, // sub left emptied by renames
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			require.NoError(t, fs.WalkDir(tt.fsys, ".", func(p string, _ fs.DirEntry, err error) error {
				require.NoError(t, err)
				got = append(got, p)
				return nil
			}))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestWalkDirSkipDir(t *testing.T) {
	fsys := MustNewFS(fstest.MapFS{
		"top/a.txt": {Data: []byte("A")},
		"z.txt":     {Data: []byte("Z")},
	})

	var got []string
	require.NoError(t, fs.WalkDir(fsys, ".", func(p string, _ fs.DirEntry, err error) error {
		require.NoError(t, err)
		got = append(got, p)
		if p == "top" {
			return fs.SkipDir // skips top's children; z.txt still visited
		}
		return nil
	}))
	assert.Equal(t, []string{".", "top", "z.txt"}, got)
}

func TestInvalidPaths(t *testing.T) {
	fsys := MustNewFS(fstest.MapFS{"a.txt": {Data: []byte("A")}})

	// fs.ErrInvalid: paths io/fs rejects outright, across every accessor.
	for _, p := range []string{"../up", "", "a//b", "/a.txt"} {
		_, openErr := fsys.Open(p)
		_, statErr := fsys.Stat(p)
		_, readDirErr := fsys.ReadDir(p)
		for op, err := range map[string]error{"Open": openErr, "Stat": statErr, "ReadDir": readDirErr} {
			require.ErrorIsf(t, err, fs.ErrInvalid, "%s(%q)", op, p)
		}
	}

	// fs.ErrNotExist: valid paths with nothing seeded there.
	for _, p := range []string{"missing.txt", "a/b.txt"} {
		_, err := fsys.Open(p)
		require.ErrorIsf(t, err, fs.ErrNotExist, p)
		_, err = fsys.Stat(p)
		require.ErrorIsf(t, err, fs.ErrNotExist, p)
		_, err = fsys.ReadDir(p)
		require.ErrorIsf(t, err, fs.ErrNotExist, p)
	}

	// Rename rejects invalid sources and destinations alike.
	err := fsys.Rename("../evil", "x.txt")
	require.ErrorContains(t, err, `rename source "../evil" not found`)
	err = fsys.Rename("missing.txt", "x.txt")
	require.ErrorContains(t, err, `rename source "missing.txt" not found`)
	for _, dest := range []string{"../evil", "", "."} {
		err := fsys.Rename("a.txt", dest)
		require.Errorf(t, err, dest)
	}
}

func TestSub(t *testing.T) {
	fsys := MustNewFS(fstest.MapFS{
		"a.txt":          {Data: []byte("A")},
		"sub/b.txt":      {Data: []byte("B")},
		"sub/deep/c.txt": {Data: []byte("C")},
	}).MustMerge(fstest.MapFS{"sub/b.txt": {Data: []byte("from B")}})

	sub, err := fsys.Sub("sub")
	require.NoError(t, err)
	require.NoError(t, fstest.TestFS(sub, "b.txt", "deep/c.txt"))

	body, err := fs.ReadFile(sub, "b.txt")
	require.NoError(t, err)
	assert.Equal(t, "from B", string(body), "content serves from its owner")

	body, err = fs.ReadFile(sub, "deep/c.txt")
	require.NoError(t, err)
	assert.Equal(t, "C", string(body))

	var got []string
	require.NoError(t, fs.WalkDir(sub, ".", func(p string, _ fs.DirEntry, err error) error {
		require.NoError(t, err)
		got = append(got, p)
		return nil
	}))
	assert.Equal(t, []string{".", "b.txt", "deep", "deep/c.txt"}, got)
}

func TestSubErrors(t *testing.T) {
	fsys := MustNewFS(fstest.MapFS{"a.txt": {Data: []byte("A")}})

	same, err := fsys.Sub(".")
	require.NoError(t, err)
	assert.Same(t, fsys, same, `"." is the filesystem itself`)

	_, err = fsys.Sub("missing.txt")
	require.ErrorIs(t, err, fs.ErrNotExist)
	_, err = fsys.Sub("a.txt")
	require.ErrorContains(t, err, "not a directory")
	for _, p := range []string{"../evil", ""} {
		_, err = fsys.Sub(p)
		require.ErrorIsf(t, err, fs.ErrInvalid, "%q", p)
	}
}

func TestNewFSError(t *testing.T) {
	fsys, err := NewFS(errFS{})
	require.Error(t, err)
	assert.Nil(t, fsys)
	assert.Panics(t, func() { MustNewFS(errFS{}) })
	assert.Panics(t, func() { MustNewFS(fstest.MapFS{}).MustMerge(errFS{}) })
}

type errFS struct{}

func (errFS) Open(string) (fs.File, error) { return nil, errors.New("boom") }

func mustRenamedFS() *FS {
	fsys := MustNewFS(fstest.MapFS{
		"a.txt":     {Data: []byte("A")},
		"sub/b.txt": {Data: []byte("B")},
	})
	if err := fsys.Rename("a.txt", "renamed.txt"); err != nil {
		panic(err) // unreachable: sources exist above
	}
	if err := fsys.Rename("sub/b.txt", "deep/c.txt"); err != nil {
		panic(err)
	}
	return fsys
}
