// Package firmrule contains ccbox's custom firm.Rules
package firmrule

import (
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
)

// Each pattern requires at least one char and rejects empty values -- no rule.Present needed.
var (
	// Domain is a lowercased hostname or its suffix, e.g. x.ai, githubusercontent.com
	Domain = rule.Match{Regexp: regexp.MustCompile(`^([a-z0-9-]+\.)*[a-z0-9-]+$`)}
	// EnvVar is a POSIX-ish env var name
	EnvVar = rule.Match{Regexp: regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)}
	// MaskDir is a project-relative dir to mask: no leading slash or "~", and no ".",
	// "..", or empty segment (".idea" and "vendor/bundle" are fine)
	MaskDir = rule.Match{Regexp: regexp.MustCompile(
		`^[.]?[^/~]*[^./~][^/~]*(/[^/~]*[^./~][^/~]*)*$`)}
	// MaskGlob is a project-relative glob of guarded paths: no leading slash, no ".." segment
	MaskGlob = rule.Match{Regexp: regexp.MustCompile(
		`^[A-Za-z0-9_.*-]*[A-Za-z0-9_*-][A-Za-z0-9_.*-]*(/[A-Za-z0-9_.*-]*[A-Za-z0-9_*-][A-Za-z0-9_.*-]*)*$`)}
	// HomePath is a $HOME-relative path, e.g. .claude or .config/opencode
	HomePath = rule.Match{Regexp: regexp.MustCompile(`^[^/].*$`)}
	// HTTPSURL is an https endpoint; download templates may carry a literal $version
	HTTPSURL = rule.Match{Regexp: regexp.MustCompile(`^https://\S+$`)}
	// bindPath is a read_only_binds host dir or container mount: absolute (/…) or
	// home-relative (~/…), segmented like MaskDir to bar ".." traversal
	bindPath = rule.Match{Regexp: regexp.MustCompile(
		`^(~/|/)([^/~]*[^./~][^/~]*)(/[^/~]*[^./~][^/~]*)*$`)}
)

const (
	// EnabledValue is read_only_binds' special value: a Specials key's entry mounts at its default
	EnabledValue = "enabled"
	bindName     = "Bind"
)

// Bind validates a read_only_binds key-value pair for rule.KeyValues
type Bind struct{ Specials []string }

// TypeCheck restricts the rule to string maps
func (Bind) TypeCheck(typ reflect.Type) *firm.RuleTypeError {
	if typ.Kind() != reflect.Map || typ.Key().Kind() != reflect.String || typ.Elem().Kind() != reflect.String {
		return firm.NewRuleTypeError(bindName, typ, "is not a Map with string keys and values")
	}
	return nil
}

// ValidateValue validates the pair (assumes TypeCheck is called)
func (b Bind) ValidateValue(value reflect.Value) firm.ErrorMap {
	binds, ok := reflect.TypeAssert[map[string]string](value)
	if !ok {
		return nil
	}
	for host, mount := range binds {
		if msg := bindEntryError(b.Specials, host, mount); msg != "" {
			return firm.ErrorMap{bindName: firm.TemplateError{Template: msg}}
		}
	}
	return nil
}

// bindEntryError describes the pair's first problem, or "" for a good pair
func bindEntryError(specials []string, host, mount string) string {
	switch {
	case slices.Contains(specials, host):
		if mount != EnabledValue {
			return "must be enabled"
		}
		return ""
	case !bindPath.Regexp.MatchString(host):
		return "must be " + strings.Join(specials, ", ") + " or a host path (~/… or /…, no .. segment)"
	case !bindPath.Regexp.MatchString(mount):
		return "must be a container mount path (~/… or /…, no .. segment)"
	default:
		return ""
	}
}
