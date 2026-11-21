// Package modelset loads model sets: the built-in models, overridden by the
// user's drop-in yaml files under the user config dir.
package modelset

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/fsutil"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/slicex"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

const yamlExt = ".yaml"

// Keyed is a model keyed by name — a user drop-in overrides a same-name built-in
type Keyed interface {
	// Key is the model's name — the override key
	Key() string
}

// userConfigDir is the user config dir: ~/.ccbox/config
var userConfigDir = userdir.ConfigDir()

// SetUserConfigDir points the user config dir at dir, redirecting every model's
// drop-ins with it; tests redirect the tree at a temp dir with it.
func SetUserConfigDir(dir string) { userConfigDir = dir }

// UserConfigDir is the user config dir the drop-ins load from: ~/.ccbox/config.
func UserConfigDir() string { return userConfigDir }

// Set is one model set: the built-in models, overridden by the user's drop-in
// yaml files, each named after its file.
type Set[T Keyed] struct {
	// Builtins are the built-in models, sorted by name
	Builtins []T
	// Dir is the drop-ins' user config subdir: "providers", "runtimes"
	Dir string
	// Parse decodes one drop-in body into its model, named after its file
	Parse func(name string, body []byte) (T, error)

	// items is the loaded set, sorted by name — the built-ins until Load
	items []T
}

// NewSet builds one model set, loaded with its built-ins.
func NewSet[T Keyed](builtins []T, dir string, parse func(name string, body []byte) (T, error)) *Set[T] {
	s := &Set[T]{Builtins: builtins, Dir: dir, Parse: parse}
	s.items = s.Builtins
	return s
}

// Load loads the set: the built-ins, overridden by the user's drop-ins.
func (s *Set[T]) Load() { s.items = s.mustLoadAll() }

// mustLoadAll merges the user drop-ins over the built-ins: a user model replaces
// a same-name built-in. Drop-in load warnings print; the set always loads.
func (s *Set[T]) mustLoadAll() []T {
	userItems, warns, err := s.LoadUser()
	for _, warn := range warns {
		log.Warnf("%s, skipping", warn)
	}
	// unreachable: listing a dir never errors
	must.Do(err)

	byName := make(map[string]T, len(s.Builtins)+len(userItems))
	for _, item := range slices.Concat(s.Builtins, userItems) {
		byName[item.Key()] = item
	}
	return slicex.Map(slices.Sorted(maps.Keys(byName)), func(name string) T { return byName[name] })
}

// LoadUser parses the user drop-ins tree, each model named after its file. A bad
// model is a warning — skipped with a load warning
func (s *Set[T]) LoadUser() ([]T, []error, error) {
	return fsutil.LoadGlob(os.DirFS(userConfigDir), s.Dir+"/*"+yamlExt,
		func(_ string, info fsutil.EntryInfo) (T, error) {
			return s.Parse(strings.TrimSuffix(info.Name(), yamlExt), info.Body)
		})
}

// LoadBuiltins loads just the built-ins, dropping the user drop-ins; tests pin
// the built-in set with it.
func (s *Set[T]) LoadBuiltins() { s.items = s.Builtins }

// All lists every known model, sorted by name
func (s *Set[T]) All() []T { return slices.Clone(s.items) }

// UserDir is the host dir of the user drop-ins: ~/.ccbox/config/<dir>.
func (s *Set[T]) UserDir() string { return filepath.Join(userConfigDir, s.Dir) }

// AliasedKeyed is a model keyed by name and addressable by allowlist alias
type AliasedKeyed interface {
	Keyed
	// Alias is the model's allowlist alias
	Alias() string
	// DomainList is the model's domains — its alias's expansion
	DomainList() []string
}

// AliasedSet is a model set whose models are addressable by allowlist alias:
// each model's own alias resolves to its domains, plus one alias for every model's.
type AliasedSet[T AliasedKeyed] struct {
	*Set[T]

	// AllAlias is the allowlist alias expanding to every model's domains
	AllAlias string
}

// Aliases lists every model's alias plus AllAlias, sorted
func (w *AliasedSet[T]) Aliases() []string {
	all := w.All()
	aliases := make([]string, 0, len(all)+1)
	for _, item := range all {
		aliases = append(aliases, item.Alias())
	}
	aliases = append(aliases, w.AllAlias)
	return slices.Sorted(slices.Values(aliases))
}

// AllDomains concatenates every model's domains
func (w *AliasedSet[T]) AllDomains() []string {
	return slices.Concat(slicex.Map(w.All(), func(item T) []string { return item.DomainList() })...)
}

// DomainsFor resolves an allowlist alias to its model's domains — every
// model's for AllAlias. ok is false for anything but an alias.
func (w *AliasedSet[T]) DomainsFor(alias string) ([]string, bool) {
	for _, item := range w.All() {
		if item.Alias() == alias {
			return slices.Clone(item.DomainList()), true
		}
	}
	if alias == w.AllAlias {
		return w.AllDomains(), true
	}
	return nil, false
}
