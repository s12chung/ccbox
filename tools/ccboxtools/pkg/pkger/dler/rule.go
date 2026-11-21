package dler

import (
	"reflect"
	"regexp"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/verify"
)

var httpsURL = rule.Match{Regexp: regexp.MustCompile(`^https://\S+$`)}

const sha512PinName = "Sha512Pin"

// sha512Pin requires a sha512 hash in the schema's declared encoding —
// "base64" (std) or "hex"; the unpinned schema (Sha512 nil) never applies it.
type sha512Pin struct{ encoding string }

// TypeCheck restricts the rule to strings.
func (sha512Pin) TypeCheck(typ reflect.Type) *firm.RuleTypeError {
	if typ.Kind() != reflect.String {
		return firm.NewRuleTypeError(sha512PinName, typ, "is not a String")
	}
	return nil
}

// ValidateValue checks the hash's encoding and size (assumes TypeCheck is called).
func (p sha512Pin) ValidateValue(value reflect.Value) firm.ErrorMap {
	if verify.DecodeSHA(verify.Sha512{Hash: value.String(), Encoding: p.encoding}) != nil {
		return nil
	}
	template := "is not a base64 std sha512"
	if p.encoding == "hex" {
		template = "is not a hex sha512"
	}
	return firm.ErrorMap{sha512PinName: firm.TemplateError{Template: template}}
}
