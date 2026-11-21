// Package firmrule contains ccbox's custom firm.Rules
package firmrule

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
)

// Each pattern requires at least one char and rejects empty values -- no rule.Present needed.
var (
	// Domain is a lowercased hostname or its suffix, e.g. x.ai, githubusercontent.com —
	// never a bare TLD: "com" would allow every .com host
	Domain = rule.Match{Regexp: regexp.MustCompile(`^([a-z0-9-]+\.)+[a-z0-9-]+$`)}
	// Name matches the name a ccbox-<name>-<kind> allowlist alias and its user
	// config file name derive from
	Name = rule.Match{Regexp: regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)}
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
	// ContainerPath is an absolute container path (/…), no ".", "..", or empty segment —
	// a runtime's PATH entry or cache volume mountpoint
	ContainerPath = rule.Match{Regexp: regexp.MustCompile(
		`^/[A-Za-z0-9._-]*[A-Za-z0-9_-][A-Za-z0-9._-]*(/[A-Za-z0-9._-]*[A-Za-z0-9_-][A-Za-z0-9._-]*)*$`)}
	// HTTPSURL is an https endpoint; download templates may carry a literal $version
	HTTPSURL = rule.Match{Regexp: regexp.MustCompile(`^https://\S+$`)}
	// bindPath is a read_only_binds host dir or container mount: absolute (/…) or
	// home-relative (~/…), segmented like MaskDir to bar ".." traversal
	bindPath = rule.Match{Regexp: regexp.MustCompile(
		`^(~/|/)([^/~]*[^./~][^/~]*)(/[^/~]*[^./~][^/~]*)*$`)}

	// aliasPrefixRule matches the reserved aliasPrefix, for DomainOrAlias's Not
	aliasPrefixRule = rule.Match{Regexp: regexp.MustCompile("^" + aliasPrefix)}
)

// Domains is a non-empty domain list — an egress wall target's domains: a
// provider's API hosts or a runtime's toolchain hosts
var Domains = []firm.Rule{rule.Len{Min: 1}, firm.Elems[[]string](Domain)}

const (
	// EnabledValue is read_only_binds' special value: a Specials key's entry mounts at its default
	EnabledValue = "enabled"
	// aliasPrefix is the allowlist aliases' reserved namespace, enforced by DomainOrAlias
	aliasPrefix = "ccbox-"

	bindName          = "Bind"
	domainOrAliasName = "DomainOrAlias"
	portListName      = "PortList"
	templateName      = "Template"
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
		if errMap := bindEntryError(b.Specials, host, mount); errMap != nil {
			return errMap
		}
	}
	return nil
}

// PortList validates a comma-separated tcp port list: each entry trimmed, a port
// 1-65535, no duplicates — "" is the unset list
type PortList struct{}

// TypeCheck restricts the rule to strings
func (PortList) TypeCheck(typ reflect.Type) *firm.RuleTypeError {
	if typ.Kind() != reflect.String {
		return firm.NewRuleTypeError(portListName, typ, "is not a string")
	}
	return nil
}

// ValidateValue validates the list (assumes TypeCheck is called)
func (PortList) ValidateValue(value reflect.Value) firm.ErrorMap {
	tcp, ok := reflect.TypeAssert[string](value)
	if !ok || tcp == "" {
		return nil
	}
	seen := map[string]bool{}
	for entry := range strings.SplitSeq(tcp, ",") {
		port := strings.TrimSpace(entry)
		portN, err := strconv.Atoi(port)
		switch {
		case err != nil || portN < 1 || portN > 65535:
			return portListErr("must be a comma-separated list of ports 1-65535, got {{.Port}}", port)
		case seen[port]:
			return portListErr("must not repeat ports, got {{.Port}}", port)
		}
		seen[port] = true
	}
	return nil
}

// portListErr names the list's bad entry, quoted so empty ones show
func portListErr(template, port string) firm.ErrorMap {
	return firm.ErrorMap{portListName: firm.TemplateError{
		TemplateFields: map[string]string{"Port": strconv.Quote(port)},
		Template:       template,
	}}
}

// Template is a text/template body that parses — a tool's postinstall cmd;
// "" is the unset tmpl
type Template struct{}

// TypeCheck restricts the rule to strings
func (Template) TypeCheck(typ reflect.Type) *firm.RuleTypeError {
	if typ.Kind() != reflect.String {
		return firm.NewRuleTypeError(templateName, typ, "is not a string")
	}
	return nil
}

// ValidateValue checks the body parses (assumes TypeCheck is called)
func (Template) ValidateValue(value reflect.Value) firm.ErrorMap {
	body, ok := reflect.TypeAssert[string](value)
	if !ok || body == "" {
		return nil
	}
	if _, err := template.New(templateName).Parse(body); err != nil {
		return firm.ErrorMap{templateName: firm.TemplateError{
			TemplateFields: map[string]string{"Error": err.Error()},
			Template:       "is not a text/template that parses: {{.Error}}",
		}}
	}
	return nil
}

// DomainOrAlias validates for domains or aliases, resolving the alias list at
// validation time — aliases load at startup (user-defined ones included)
func DomainOrAlias(aliasesFunc func() []string) firm.RuleBasic {
	return rule.CustomizeErr(
		rule.Or(
			rule.OneOf[string]{ValuesFunc: aliasesFunc},
			// the AND NOT reserves the ccbox- namespace: a dotted ccbox-* string
			// would otherwise pass Domain while reading as an alias
			rule.And(rule.Not(aliasPrefixRule), Domain),
		),
		func(firm.ErrorMap) firm.ErrorMap {
			return firm.ErrorMap{domainOrAliasName: firm.TemplateError{
				TemplateFields: map[string]string{"Aliases": quotedValues(aliasesFunc())},
				Template:       "is not a domain or one of {{.Aliases}}",
			}}
		},
	)
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

// bindEntryError returns the pair's specific TemplateError, or nil for a good pair
func bindEntryError(specials []string, host, mount string) firm.ErrorMap {
	switch {
	case slices.Contains(specials, host):
		if mount != EnabledValue {
			return bindErr("must be enabled", mount)
		}
		return nil
	case !bindPath.Regexp.MatchString(host):
		return bindErr("must be "+strings.Join(specials, ", ")+" or a host path (~/… or /…, no .. segment)", mount)
	case !bindPath.Regexp.MatchString(mount):
		return bindErr("must be a container mount path (~/… or /…, no .. segment)", mount)
	default:
		return nil
	}
}

// bindErr names the pair's bad mount, quoted so empty values show — the error key already names the host
func bindErr(requirement, mount string) firm.ErrorMap {
	return firm.ErrorMap{bindName: firm.TemplateError{
		TemplateFields: map[string]string{"Mount": strconv.Quote(mount)},
		Template:       requirement + ": {{.Mount}}",
	}}
}
