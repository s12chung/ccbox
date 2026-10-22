// Package termlint lints repo text files for banned terms configured in a
// JSON config (make lint.terms).
package termlint

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gobwas/glob"

	"github.com/s12chung/ccbox/tools/termlint/pkg/config"
	"github.com/s12chung/ccbox/tools/termlint/pkg/pattern"
)

type term struct {
	config.Term

	order   int
	matcher pattern.Matcher
}

type violation struct {
	term *term
	path string
	line int
	text string
}

// Run lints the text files under root for the terms configured in root's
// .termlint.json. It returns an error reporting every violation, nil when the
// repo is clean.
func Run(root string) error {
	cfg, err := config.Load(filepath.Join(root, ".termlint.json"))
	if err != nil {
		return err
	}

	excludes, err := compileGlobs(cfg.ExcludeGlobs)
	if err != nil {
		return err
	}
	violations, err := scan(root, cfg.Extensions, excludes, compileTerms(cfg.Terms))
	if err != nil {
		return err
	}
	if len(violations) == 0 {
		return nil
	}
	return errors.New(report(violations))
}

func compileTerms(terms []config.Term) []*term {
	compiled := make([]*term, 0, len(terms))
	for i, t := range terms {
		compiled = append(compiled, &term{Term: t, order: i, matcher: pattern.Compile(t.Word, t.Allows)})
	}
	return compiled
}

func compileGlobs(patterns []string) ([]glob.Glob, error) {
	globs := make([]glob.Glob, 0, len(patterns))
	for _, pattern := range patterns {
		compiled, err := glob.Compile(pattern, '/')
		if err != nil {
			return nil, err
		}
		globs = append(globs, compiled)
	}
	return globs, nil
}

func scan(root string, extensions []string, excludes []glob.Glob, terms []*term) ([]violation, error) {
	gitDir := filepath.Join(root, ".git")
	var violations []violation
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == gitDir {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !slices.Contains(extensions, filepath.Ext(rel)) || slices.ContainsFunc(excludes, func(g glob.Glob) bool {
			return g.Match(rel)
		}) {
			return nil
		}
		content, err := os.ReadFile(path) // #nosec G304,G122 -- the linter's own repo checkout
		if err != nil {
			return err
		}
		violations = append(violations, scanContent(rel, string(content), terms)...)
		return nil
	})
	return violations, err
}

func scanContent(path, content string, terms []*term) []violation {
	var violations []violation
	for i, text := range strings.Split(content, "\n") {
		for _, t := range terms {
			if t.matcher.IsHit(text) {
				violations = append(violations, violation{t, path, i + 1, strings.TrimSpace(text)})
			}
		}
	}
	return violations
}

// report groups violations by term, each group followed by its term's hint.
func report(violations []violation) string {
	slices.SortStableFunc(violations, func(a, b violation) int { return cmp.Compare(a.term.order, b.term.order) })
	b := &strings.Builder{}
	var previous *term
	for _, v := range violations {
		if previous != nil && v.term != previous {
			b.WriteString("\n" + previous.Hint + "\n")
		}
		fmt.Fprintf(b, "%s:%d:%s\n", v.path, v.line, v.text)
		previous = v.term
	}
	b.WriteString("\n" + previous.Hint)
	return b.String()
}
