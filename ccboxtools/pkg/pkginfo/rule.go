package pkginfo

import (
	"reflect"

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
