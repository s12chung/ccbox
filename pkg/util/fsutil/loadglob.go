package fsutil

import (
	"errors"
	"io/fs"
)

// EntryInfo is a glob match: its stat plus, for a file, its content — a dir
// match has no Body.
type EntryInfo struct {
	fs.FileInfo

	Body []byte
}

// Skip is returned by a LoadGlob load func to drop the match being loaded,
// like fs.SkipDir — it is never surfaced as a warning.
var Skip = errors.New("fsutil: skip") //nolint:errname,revive,staticcheck // named after fs.SkipDir

// LoadGlob globs fsys for pattern and loads each match with load. A glob error
// is returned — it means a malformed pattern, so fail loudly rather than warn.
// A stat, read, or load error is a warning: the entry is skipped, and the
// warnings are returned for callers to surface. load returns Skip to drop a
// match silently.
func LoadGlob[T any](fsys fs.FS, pattern string, load func(p string, info EntryInfo) (t T, err error)) ([]T, []error, error) {
	paths, err := fs.Glob(fsys, pattern)
	if err != nil {
		// malformed pattern: the only fs.Glob error — I/O problems surface as
		// no matches, then as stat/read warnings below
		return nil, nil, err
	}

	loaded := make([]T, 0, len(paths))
	var warns []error
	for _, p := range paths {
		entry, err := entryInfo(fsys, p)
		if err != nil {
			warns = append(warns, err)
			continue
		}
		t, err := load(p, entry)
		if errors.Is(err, Skip) {
			continue
		}
		if err != nil {
			warns = append(warns, err)
			continue
		}
		loaded = append(loaded, t)
	}
	return loaded, warns, nil
}

// entryInfo stats one match and reads it, unless it is a dir
func entryInfo(fsys fs.FS, p string) (EntryInfo, error) {
	info, err := fs.Stat(fsys, p)
	if err != nil {
		return EntryInfo{}, err
	}
	entry := EntryInfo{FileInfo: info}
	if !info.IsDir() {
		entry.Body, err = fs.ReadFile(fsys, p)
	}
	return entry, err
}
