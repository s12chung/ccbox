// Package firmrule contains ccbox's custom firm.Rules
package firmrule

import (
	"reflect"
	"regexp"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"

	"github.com/s12chung/ccbox/pkg/kit/git"
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
)

const hasValidGitDirName = "HasValidGitDir"

// HasValidGitDir checks a host_git_config bool: when enabled, the host's git config dir must be valid
type HasValidGitDir struct{}

// ValidateValue checks the indirected bool (assumes TypeCheck is called)
func (HasValidGitDir) ValidateValue(value reflect.Value) firm.ErrorMap {
	if !value.Bool() {
		return nil
	}
	if _, err := git.XDGConfigDir(); err != nil {
		return firm.ErrorMap{hasValidGitDirName: firm.TemplateError{
			Template:       "requires a resolvable host git config dir: {{.Err}}",
			TemplateFields: map[string]string{"Err": err.Error()},
		}}
	}
	return nil
}

// TypeCheck restricts the rule to bools
func (HasValidGitDir) TypeCheck(typ reflect.Type) *firm.RuleTypeError {
	if typ.Kind() != reflect.Bool {
		return firm.NewRuleTypeError(hasValidGitDirName, typ, "is not a Bool")
	}
	return nil
}
