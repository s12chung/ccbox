// Package firmrule contains ccbox's custom firm.Rules
package firmrule

import (
	"fmt"
	"reflect"
	"regexp"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
)

// Each pattern requires at least one char and rejects empty values -- no rule.Present needed.
var (
	// Domain is a lowercased hostname or its suffix, e.g. x.ai, githubusercontent.com
	Domain = rule.Match{Regexp: regexp.MustCompile(`^([a-z0-9-]+\.)*[a-z0-9-]+$`)}
	// EnvVar is a POSIX-ish env var name
	EnvVar = rule.Match{Regexp: regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)}
	// MaskDir is a project-relative dir to mask: no leading slash or ".." (".idea" is fine)
	MaskDir = rule.Match{Regexp: regexp.MustCompile(`^[.]?[^./]`)}
	// HomePath is a $HOME-relative path, e.g. .claude or .config/opencode
	HomePath = rule.Match{Regexp: regexp.MustCompile(`^[^/].*$`)}
	// FileName is a bare filename, no directories
	FileName = rule.Match{Regexp: regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)}
	// NpmPackage is an npm package name, scoped or not, e.g. @anthropic-ai/claude-code
	NpmPackage = rule.Match{Regexp: regexp.MustCompile(`^(@[a-z0-9-]+/)?[a-z0-9][a-z0-9._-]*$`)}
	// HTTPSURL is an https endpoint; download templates may carry a literal $version
	HTTPSURL = rule.Match{Regexp: regexp.MustCompile(`^https://\S+$`)}
)

const definedOnceName = "DefinedOnce"

// DefinedOnce requires exactly one of the named pointer fields to be set
type DefinedOnce struct{ Fields []string }

// ValidateValue counts the named fields (assumes TypeCheck is called)
func (e DefinedOnce) ValidateValue(value reflect.Value) firm.ErrorMap {
	set := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		if !value.FieldByName(f).IsNil() {
			set = append(set, f)
		}
	}
	if len(set) == 1 {
		return nil
	}
	return firm.ErrorMap{definedOnceName: firm.TemplateError{
		Template:       "want exactly one of {{.Fields}}, got {{.Set}}",
		TemplateFields: map[string]string{"Fields": fmt.Sprintf("%v", e.Fields), "Set": fmt.Sprintf("%v", set)},
	}}
}

// TypeCheck requires a struct carrying all named fields as pointers
func (e DefinedOnce) TypeCheck(typ reflect.Type) *firm.RuleTypeError {
	if typ.Kind() != reflect.Struct {
		return firm.NewRuleTypeError(definedOnceName, typ, "is not a Struct")
	}
	for _, f := range e.Fields {
		field, ok := typ.FieldByName(f)
		if !ok || field.Type.Kind() != reflect.Pointer {
			return firm.NewRuleTypeError(definedOnceName, typ, "has no pointer field, "+f)
		}
	}
	return nil
}
