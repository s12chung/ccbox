package jsonutil

import (
	"testing"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// body is firm-registered so Decode's validation branch has a rule to trip
type body struct {
	Kind string `json:"kind"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[body]().
		Validates(firm.RuleMap{
			"Kind": {rule.Equal[string]{To: "ok"}},
		}))
}

func TestDecode(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := Decode[body](`{"kind":"ok"}`)

		require.NoError(t, err)
		assert.Equal(t, body{Kind: "ok"}, got)
	})

	t.Run("unknown field", func(t *testing.T) {
		_, err := Decode[body](`{"kind":"ok","extra":1}`)

		require.Error(t, err)
		assert.ErrorContains(t, err, "unknown field")
	})

	t.Run("bad json names the body", func(t *testing.T) {
		_, err := Decode[body](`{"kind":`)

		require.Error(t, err)
		assert.ErrorContains(t, err, `parse {"kind":`)
	})

	t.Run("invalid value", func(t *testing.T) {
		_, err := Decode[body](`{"kind":"no"}`)

		require.Error(t, err)
		assert.ErrorContains(t, err, "Kind.Equal")
	})
}
