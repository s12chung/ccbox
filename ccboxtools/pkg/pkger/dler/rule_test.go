package dler

import (
	"crypto/sha512"
	"encoding/base64"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBase64Sha512_TypeCheck(t *testing.T) {
	assert.Nil(t, base64Sha512{}.TypeCheck(reflect.TypeFor[string]()))

	for _, typ := range []reflect.Type{reflect.TypeFor[int](), reflect.TypeFor[bool](), reflect.TypeFor[[]string]()} {
		err := base64Sha512{}.TypeCheck(typ)
		require.NotNilf(t, err, "want %s rejected", typ)
		assert.Equalf(t, "Base64Sha512: value is not a String, got "+typ.String(), err.Error(), "%s", typ)
	}
}

func TestBase64Sha512_ValidateValue(t *testing.T) {
	valid := []string{
		"", // the optional selector
		base64.StdEncoding.EncodeToString(make([]byte, sha512.Size)),
	}
	invalid := []string{
		"not base64",
		base64.StdEncoding.EncodeToString(make([]byte, sha512.Size256)), // decodes, wrong size
		base64.RawStdEncoding.EncodeToString(make([]byte, sha512.Size)), // right size, no padding
	}
	for _, s := range valid {
		assert.Nilf(t, base64Sha512{}.ValidateValue(reflect.ValueOf(s)).ToNil(), "want %q valid", s)
	}
	for _, s := range invalid {
		errMap := base64Sha512{}.ValidateValue(reflect.ValueOf(s))
		require.NotEmptyf(t, errMap, "want %q invalid", s)
		assert.Containsf(t, errMap.Error(), "is not a base64 std sha512", "%q", s)
	}
}

func TestHttpsURL(t *testing.T) {
	for _, s := range []string{"https://example.com", "https://example.com/a?b=c"} {
		assert.Nilf(t, httpsURL.ValidateValue(reflect.ValueOf(s)).ToNil(), "want %q valid", s)
	}
	for _, s := range []string{"http://example.com", "example.com", "https://", "https://a b"} {
		assert.NotEmptyf(t, httpsURL.ValidateValue(reflect.ValueOf(s)), "want %q invalid", s)
	}
}
