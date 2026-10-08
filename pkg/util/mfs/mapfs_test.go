// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mfs

import (
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapFS(t *testing.T) {
	m := MapFS{
		"hello":             {Data: []byte("hello, world\n")},
		"fortune/k/ken.txt": {Data: []byte("If a program is too slow, it must have a loop.\n")},
	}
	if err := fstest.TestFS(m, "hello", "fortune", "fortune/k", "fortune/k/ken.txt"); err != nil {
		t.Fatal(err)
	}
}

func TestMapFSChmodDot(t *testing.T) {
	m := MapFS{
		"a/b.txt": &MapFile{Mode: 0o666},
		".":       &MapFile{Mode: 0o777 | fs.ModeDir},
	}
	buf := new(strings.Builder)
	require.NoError(t, fs.WalkDir(m, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintf(buf, "%s: %v\n", path, fi.Mode())
		return nil
	}))
	want := `
.: drwxrwxrwx
a: dr-xr-xr-x
a/b.txt: -rw-rw-rw-
`[1:]
	got := buf.String()
	if want != got {
		t.Errorf("MapFS modes want:\n%s\ngot:\n%s\n", want, got)
	}
}

func TestMapFSFileInfoName(t *testing.T) {
	m := MapFS{
		"path/to/b.txt": &MapFile{},
	}
	info, _ := m.Stat("path/to/b.txt")
	want := "b.txt"
	got := info.Name()
	if want != got {
		t.Errorf("MapFS FileInfo.Name want:\n%s\ngot:\n%s\n", want, got)
	}
}

//nolint:cyclop // verbatim from Go's fstest tests
func TestMapFSSymlink(t *testing.T) {
	const fileContent = "If a program is too slow, it must have a loop.\n"
	m := MapFS{
		"fortune/k/ken.txt": {Data: []byte(fileContent)},
		"dirlink":           {Data: []byte("fortune/k"), Mode: fs.ModeSymlink},
		"linklink":          {Data: []byte("dirlink"), Mode: fs.ModeSymlink},
		"ken.txt":           {Data: []byte("dirlink/ken.txt"), Mode: fs.ModeSymlink},
	}
	if err := fstest.TestFS(m, "fortune/k/ken.txt", "dirlink", "ken.txt", "linklink"); err != nil {
		t.Error(err)
	}

	gotData, err := fs.ReadFile(m, "ken.txt")
	if string(gotData) != fileContent || err != nil {
		t.Errorf("fs.ReadFile(m, \"ken.txt\") = %q, %v; want %q, <nil>", gotData, err, fileContent)
	}
	gotLink, err := fs.ReadLink(m, "dirlink")
	if want := "fortune/k"; gotLink != want || err != nil {
		t.Errorf("fs.ReadLink(m, \"dirlink\") = %q, %v; want %q, <nil>", gotLink, err, fileContent)
	}
	gotInfo, err := fs.Lstat(m, "dirlink")
	if err != nil {
		t.Errorf("fs.Lstat(m, \"dirlink\") = _, %v; want _, <nil>", err)
	} else {
		if got, want := gotInfo.Name(), "dirlink"; got != want {
			t.Errorf("fs.Lstat(m, \"dirlink\").Name() = %q; want %q", got, want)
		}
		if got, want := gotInfo.Mode(), fs.ModeSymlink; got != want {
			t.Errorf("fs.Lstat(m, \"dirlink\").Mode() = %v; want %v", got, want)
		}
	}
	gotInfo, err = fs.Stat(m, "dirlink")
	if err != nil {
		t.Errorf("fs.Stat(m, \"dirlink\") = _, %v; want _, <nil>", err)
	} else {
		if got, want := gotInfo.Name(), "dirlink"; got != want {
			t.Errorf("fs.Stat(m, \"dirlink\").Name() = %q; want %q", got, want)
		}
		if got, want := gotInfo.Mode(), fs.ModeDir|0o555; got != want {
			t.Errorf("fs.Stat(m, \"dirlink\").Mode() = %v; want %v", got, want)
		}
	}
	gotInfo, err = fs.Lstat(m, "linklink")
	if err != nil {
		t.Errorf("fs.Lstat(m, \"linklink\") = _, %v; want _, <nil>", err)
	} else {
		if got, want := gotInfo.Name(), "linklink"; got != want {
			t.Errorf("fs.Lstat(m, \"linklink\").Name() = %q; want %q", got, want)
		}
		if got, want := gotInfo.Mode(), fs.ModeSymlink; got != want {
			t.Errorf("fs.Lstat(m, \"linklink\").Mode() = %v; want %v", got, want)
		}
	}
	gotInfo, err = fs.Stat(m, "linklink")
	if err != nil {
		t.Errorf("fs.Stat(m, \"linklink\") = _, %v; want _, <nil>", err)
	} else {
		if got, want := gotInfo.Name(), "linklink"; got != want {
			t.Errorf("fs.Stat(m, \"linklink\").Name() = %q; want %q", got, want)
		}
		if got, want := gotInfo.Mode(), fs.ModeDir|0o555; got != want {
			t.Errorf("fs.Stat(m, \"linklink\").Mode() = %v; want %v", got, want)
		}
	}
}

func TestMapFS_MergeOwnsPaths(t *testing.T) {
	base := fstest.MapFS{
		"shared.txt":   {Data: []byte("from base")},
		"base-only":    {Data: []byte("B")},
		"sub/nested":   {Data: []byte("nested")},
		"sub/deep.txt": {Data: []byte("from base")},
	}
	overlay := MapFS{
		"shared.txt":   {Data: []byte("from overlay")},
		"sub/deep.txt": {Data: []byte("from overlay")},
		"overlay-only": {Data: []byte("O")},
	}

	fsys := MustNewFS(base).MustMerge(overlay)

	require.NoError(t, fstest.TestFS(fsys, "shared.txt", "base-only", "sub/nested", "sub/deep.txt", "overlay-only"))

	for path, want := range map[string]string{
		"shared.txt":   "from overlay",
		"sub/deep.txt": "from overlay",
		"base-only":    "B",
		"sub/nested":   "nested",
		"overlay-only": "O",
	} {
		got, err := fs.ReadFile(fsys, path)
		require.NoErrorf(t, err, "reading %q", path)
		assert.Equalf(t, want, string(got), "merged %q", path)
	}
}
