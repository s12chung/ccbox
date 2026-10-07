// Package globkit wraps gobwas/glob with ccbox's project-glob idioms.
package globkit

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gobwas/glob"
)

// DoubleStarRooted compiles g to a glob that also matches at the root when g starts with
// "**/": the stripped twin, so "**/*.pem" covers "server.pem" too. Compiles as-is otherwise.
func DoubleStarRooted(g string) glob.Glob {
	compiled := glob.MustCompile(g, '/')
	twin, ok := strings.CutPrefix(g, "**/")
	if !ok {
		return compiled
	}
	return doubleStarRooted{compiled, glob.MustCompile(twin, '/')}
}

// doubleStarRooted matches its glob or the twin. MaskGlob's charset leaves "*" the only
// special char, so the compiles are unreachable panics.
type doubleStarRooted struct {
	glob.Glob

	twin glob.Glob
}

func (d doubleStarRooted) Match(s string) bool { return d.Glob.Match(s) || d.twin.Match(s) }

// MatchLimit is the match count at which a project-wide glob walk reads as a strayed run
// — at home or /, the project globs match everywhere
const MatchLimit = 50

// WalkMatches walks root once, returning each present matching path (files or dirs), in
// walk order
func WalkMatches(root string, matchers ...glob.Glob) []string {
	return walkMatches(root, 0, matchers...) // 0 walks unlimited
}

// ManyMatches reports whether walking root yields MatchLimit matches or more, quitting
// the walk at the limit
func ManyMatches(root string, matchers ...glob.Glob) bool {
	return len(walkMatches(root, MatchLimit, matchers...)) >= MatchLimit
}

// walkMatches is WalkMatches, quitting the walk once len(matches) hits a nonzero limit
func walkMatches(root string, limit int, matchers ...glob.Glob) []string {
	var matches []string
	_ = filepath.WalkDir(root, func(p string, _ fs.DirEntry, err error) error {
		rel, relErr := filepath.Rel(root, p)
		if err != nil || relErr != nil || rel == "." {
			return nil //nolint:nilerr // unstatable/unrelativable paths just don't match
		}
		if !slices.ContainsFunc(matchers, func(m glob.Glob) bool { return m.Match(filepath.ToSlash(rel)) }) {
			return nil
		}
		matches = append(matches, rel)
		if limit > 0 && len(matches) == limit {
			return fs.SkipAll // the limit speaks: no need to keep walking
		}
		return nil
	})
	return matches
}
