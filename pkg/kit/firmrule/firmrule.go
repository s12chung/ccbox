// Package firmrule contains ccbox's custom firm.Rules
package firmrule

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
)

// Each pattern requires at least one char and rejects empty values -- no rule.Present needed.
var (
	// Domain is a lowercased hostname or its suffix, e.g. x.ai, githubusercontent.com
	Domain = rule.Match{Regexp: regexp.MustCompile(`^([a-z0-9-]+\.)*[a-z0-9-]+$`)} // EnvVar is a POSIX-ish env var name
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

	// aliasPrefixRule matches the reserved aliasPrefix, for DomainOrAlias's Not
	aliasPrefixRule = rule.Match{Regexp: regexp.MustCompile("^" + aliasPrefix)}
)

const (
	// EnabledValue is read_only_binds' special value: a Specials key's entry mounts at its default
	EnabledValue = "enabled"
	// aliasPrefix is the allowlist aliases' reserved namespace, enforced by DomainOrAlias
	aliasPrefix = "ccbox-"

	bindName          = "Bind"
	domainOrAliasName = "DomainOrAlias"
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

// DomainOrAlias validates for domains or aliases, resolving the alias list at
// validation time — aliases load at startup (user-defined ones included)
func DomainOrAlias(aliasesFunc func() []string) firm.RuleBasic {
	return rule.ErrCustomized{
		Rule: rule.Or{
			Rules: []firm.RuleBasic{
				rule.OneOfFunc[string]{ValuesFunc: aliasesFunc},
				rule.And{
					Rules: []firm.RuleBasic{
						rule.Not{Rule: aliasPrefixRule}, // alias syntax fits with domains
						Domain,
					},
				},
			},
		},
		CustomErr: func(firm.ErrorMap) firm.ErrorMap {
			return firm.ErrorMap{domainOrAliasName: firm.TemplateError{
				TemplateFields: map[string]string{"Aliases": quotedValues(aliasesFunc())},
				Template:       "is not a domain or one of {{.Aliases}}",
			}}
		},
	}
}

// quotedValues renders values for an error message, quoted like firm's OneOf: an
// empty string must show. It mirrors firm's unexported valuesStr.
func quotedValues[T comparable](values []T) string {
	strs := make([]string, len(values))
	for i, v := range values {
		if s, ok := any(v).(string); ok {
			strs[i] = strconv.Quote(s)
		} else {
			strs[i] = fmt.Sprintf("%v", v)
		}
	}
	return fmt.Sprintf("%v", strs)
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
