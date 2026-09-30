package pkginfo

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJQExpr_TypeCheck(t *testing.T) {
	assert.Nil(t, jqExpr{}.TypeCheck(reflect.TypeFor[string]()))

	for _, typ := range []reflect.Type{reflect.TypeFor[int](), reflect.TypeFor[bool](), reflect.TypeFor[[]string]()} {
		err := jqExpr{}.TypeCheck(typ)
		require.NotNilf(t, err, "want %s rejected", typ)
		assert.Equalf(t, "JQExpr: value is not a String, got "+typ.String(), err.Error(), "%s", typ)
	}
}

func TestJQExpr_ValidateValue(t *testing.T) {
	valid := []string{
		".",
		".version",
		".path.to.version",
		".[0]",
		".[\"a b\"]",
		".version // \"latest\"",
		"$x",
		"", // gojq parses the empty program; emptiness is rule.Present's concern
	}
	invalid := []string{
		".version]",
		"{",
		".version..",
	}
	for _, s := range valid {
		assert.Nilf(t, jqExpr{}.ValidateValue(reflect.ValueOf(s)).ToNil(), "want %q valid", s)
	}
	for _, s := range invalid {
		errMap := jqExpr{}.ValidateValue(reflect.ValueOf(s))
		require.NotEmptyf(t, errMap, "want %q invalid", s)
		assert.Containsf(t, errMap.Error(), "is not a valid jq expression", "%q", s)
	}

	// the rendered message carries the underlying gojq parse error
	errMap := jqExpr{}.ValidateValue(reflect.ValueOf(".version]"))
	assert.Contains(t, errMap.Error(), `unexpected token "]"`)
}
