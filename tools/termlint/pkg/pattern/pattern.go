// Package pattern builds case-insensitive line matchers for banned terms and
// their allowed phrases.
package pattern

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
)

// Matcher reports whether a line has a term match that no allow overlaps.
type Matcher struct {
	term   *regexp.Regexp
	allows []*regexp.Regexp
}

// Compile matches word's generated forms with its allowed phrases.
func Compile(word string, allows []string) Matcher {
	matcher := Matcher{term: termPattern(word)}
	for _, allow := range allows {
		matcher.allows = append(matcher.allows, allowPattern(allow))
	}
	return matcher
}

// termPattern matches word's forms in any casing, bounded by a word boundary
// or an underscore (wall, WALL, my_walls), plus camelCase: a lowercase letter
// against the capitalized form (NewWall, egressWall). The camel alternative is
// case-sensitive on purpose: the capital is what marks an identifier, keeping
// plain words like brides or striding out of ride's matches. Matches keep the
// bounding letter so overlaps against allows include it.
func termPattern(word string) *regexp.Regexp {
	wordForms := forms(word)
	capForms := make([]string, len(wordForms))
	for i, form := range wordForms {
		capForms[i] = strings.ToUpper(form[:1]) + form[1:]
	}
	// longest first, so alternation claims walls before leaving a boundary-less wall
	return regexp.MustCompile(
		`(?i)(?:\b|_)(?:` + strings.Join(wordForms, "|") + `)(?:\b|_)` +
			`|(?-i:[a-z](?:` + strings.Join(capForms, "|") + `)(?:\b|_))`)
}

// forms generates word's plural and past/participle forms, longest first:
// wall → walling, walled, walls, wall; ride → riding, rides, rided, ride.
func forms(word string) []string {
	forms := []string{word, word + "s"}
	if stem, isE := strings.CutSuffix(word, "e"); isE {
		forms = append(forms, stem+"ed", stem+"ing") // ride → riding, not rideing
	} else {
		forms = append(forms, word+"ed", word+"ing")
	}
	slices.SortStableFunc(forms, func(a, b string) int { return cmp.Compare(len(b), len(a)) })
	return forms
}

// allowPattern matches phrase's words separated by any run of non-alphanumerics
// (or none), e.g. "egress wall" also matches egressWall, egress-wall, egress_wall.
func allowPattern(phrase string) *regexp.Regexp {
	words := Words(phrase)
	parts := make([]string, len(words))
	for i, word := range words {
		parts[i] = regexp.QuoteMeta(word)
	}
	return regexp.MustCompile(`(?i)` + strings.Join(parts, `[^a-zA-Z0-9]*`))
}

// IsHit reports whether line has a term match that no allow overlaps.
func (m Matcher) IsHit(line string) bool {
	allowed := make([][]int, 0, len(m.allows))
	for _, allow := range m.allows {
		allowed = append(allowed, allow.FindAllStringIndex(line, -1)...)
	}
	for _, hit := range m.term.FindAllStringIndex(line, -1) {
		if !slices.ContainsFunc(allowed, func(span []int) bool { return overlaps(hit, span) }) {
			return true
		}
	}
	return false
}

func overlaps(a, b []int) bool { return a[0] < b[1] && b[0] < a[1] }

// Words splits phrase on spaces and slashes: "wall/walled" → wall, walled.
func Words(phrase string) []string {
	return strings.FieldsFunc(phrase, func(r rune) bool { return r == ' ' || r == '/' })
}
