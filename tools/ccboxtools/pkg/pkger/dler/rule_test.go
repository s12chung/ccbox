package dler

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHttpsURL(t *testing.T) {
	for _, s := range []string{"https://example.com", "https://example.com/a?b=c"} {
		assert.Nilf(t, httpsURL.ValidateValue(reflect.ValueOf(s)).ToNil(), "want %q valid", s)
	}
	for _, s := range []string{"http://example.com", "example.com", "https://", "https://a b"} {
		assert.NotEmptyf(t, httpsURL.ValidateValue(reflect.ValueOf(s)), "want %q invalid", s)
	}
}

func TestSha512Pin_TypeCheck(t *testing.T) {
	assert.Nil(t, sha512Pin{}.TypeCheck(reflect.TypeFor[string]()))

	for _, typ := range []reflect.Type{reflect.TypeFor[int](), reflect.TypeFor[bool](), reflect.TypeFor[[]string]()} {
		err := sha512Pin{}.TypeCheck(typ)
		require.NotNilf(t, err, "want %s rejected", typ)
		assert.Equalf(t, "Sha512Pin: value is not a String, got "+typ.String(), err.Error(), "%s", typ)
	}
}

func TestSha512Pin_ValidateValue(t *testing.T) {
	tests := []struct {
		encoding string
		valid    []string
		invalid  []string
	}{
		{
			encoding: "base64",
			valid: []string{
				base64.StdEncoding.EncodeToString(make([]byte, sha512.Size)),
			},
			invalid: []string{
				"", // the manifest pinned nothing: the schema's nil Sha512 skips the rule
				"not base64",
				base64.StdEncoding.EncodeToString(make([]byte, sha512.Size256)), // decodes, wrong size
				base64.RawStdEncoding.EncodeToString(make([]byte, sha512.Size)), // right size, no padding
				hex.EncodeToString(make([]byte, sha512.Size)),                   // hex hash, wrong encoding
			},
		},
		{
			encoding: "hex",
			valid: []string{
				hex.EncodeToString(make([]byte, sha512.Size)),
			},
			invalid: []string{
				"", // the manifest pinned nothing: the schema's nil Sha512 skips the rule
				base64.StdEncoding.EncodeToString(make([]byte, sha512.Size)), // base64 hash, wrong encoding
				hex.EncodeToString(make([]byte, sha512.Size256)),             // hex, wrong size
			},
		},
	}
	for _, tt := range tests {
		for _, s := range tt.valid {
			assert.Nilf(t, sha512Pin{encoding: tt.encoding}.ValidateValue(reflect.ValueOf(s)).ToNil(), "want %q valid", s)
		}
		for _, s := range tt.invalid {
			errMap := sha512Pin{encoding: tt.encoding}.ValidateValue(reflect.ValueOf(s))
			require.NotEmptyf(t, errMap, "want %q invalid", s)
			want := "is not a base64 std sha512"
			if tt.encoding == "hex" {
				want = "is not a hex sha512"
			}
			assert.Containsf(t, errMap.Error(), want, "%q", s)
		}
	}
}
