package dler

import (
	"crypto/sha512"
	"encoding/base64"
	"reflect"
	"regexp"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
)

var httpsURL = rule.Match{Regexp: regexp.MustCompile(`^https://\S+$`)}

const base64Sha512Name = "Base64Sha512"

// base64Sha512 requires a base64 std-encoded sha512 hash (the one pinned
// encoding, per verify.Sha512Reader) while keeping the empty selector optional.
type base64Sha512 struct{}

// TypeCheck restricts the rule to strings.
func (base64Sha512) TypeCheck(typ reflect.Type) *firm.RuleTypeError {
	if typ.Kind() != reflect.String {
		return firm.NewRuleTypeError(base64Sha512Name, typ, "is not a String")
	}
	return nil
}

// ValidateValue checks the hash's encoding and size (assumes TypeCheck is called).
func (base64Sha512) ValidateValue(value reflect.Value) firm.ErrorMap {
	if value.String() == "" { // the optional selector
		return nil
	}
	sum, err := base64.StdEncoding.DecodeString(value.String())
	if err == nil && len(sum) == sha512.Size {
		return nil
	}
	return firm.ErrorMap{base64Sha512Name: firm.TemplateError{
		Template: "is not a base64 std sha512",
	}}
}
