package pkginfo

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"

	"github.com/itchyny/gojq"
	"github.com/s12chung/firm"
)

const jqExprName = "JQExpr"

// jqExpr is a jq selector: gojq parses it at validation — host CLI.yaml loads
// reject syntax errors at startup; its semantics stay a runtime check.
type jqExpr struct{}

// TypeCheck restricts the rule to strings.
func (jqExpr) TypeCheck(typ reflect.Type) *firm.RuleTypeError {
	if typ.Kind() != reflect.String {
		return firm.NewRuleTypeError(jqExprName, typ, "is not a String")
	}
	return nil
}

// ValidateValue validates the selector's syntax (assumes TypeCheck is called).
func (jqExpr) ValidateValue(value reflect.Value) firm.ErrorMap {
	if _, err := gojq.Parse(value.String()); err != nil {
		return firm.ErrorMap{jqExprName: firm.TemplateError{
			Template:       "is not a valid jq expression: {{.Err}}",
			TemplateFields: map[string]string{"Err": err.Error()},
		}}
	}
	return nil
}

var wxhRe = regexp.MustCompile(`^([0-9]+)x([0-9]+)$`)

// The wxh rule's name and the resolution's bounds — what the desktop's Xvnc geometry accepts
const (
	wxhName = "WxH"
	wxhMin  = 32
	wxhMax  = 16384
)

// wxh is a VNC box resolution "WxH" (e.g. 1600x900) within the desktop's bounds
type wxh struct{}

// TypeCheck restricts the rule to strings.
func (wxh) TypeCheck(typ reflect.Type) *firm.RuleTypeError {
	if typ.Kind() != reflect.String {
		return firm.NewRuleTypeError(wxhName, typ, "is not a String")
	}
	return nil
}

// ValidateValue validates the resolution (assumes TypeCheck is called).
func (wxh) ValidateValue(value reflect.Value) firm.ErrorMap {
	s, ok := reflect.TypeAssert[string](value)
	if !ok {
		return nil
	}
	m := wxhRe.FindStringSubmatch(s)
	if m == nil {
		return wxhError("must be WxH, e.g. 1600x900")
	}
	w, werr := strconv.Atoi(m[1])
	h, herr := strconv.Atoi(m[2])
	if werr != nil || herr != nil || w < wxhMin || w > wxhMax || h < wxhMin || h > wxhMax {
		return wxhError(fmt.Sprintf("each side must be within %d..%d", wxhMin, wxhMax))
	}
	return nil
}

// wxhError is the rule's ErrorMap for one template
func wxhError(template string) firm.ErrorMap {
	return firm.ErrorMap{wxhName: firm.TemplateError{Template: template}}
}
