package errs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSwallow(t *testing.T) {
	target := errors.New("target")
	other := errors.New("other")
	for _, tc := range []struct {
		caseName string
		err      error
		targets  []error
		want     error
	}{
		{"PassesNil", nil, []error{target}, nil},
		{"SwallowsMatch", target, []error{target}, nil},
		{"SwallowsWrapped", fmt.Errorf("wrapped: %w", target), []error{target}, nil},
		{"SwallowsAnyOfMany", other, []error{target, other}, nil},
		{"PassesOther", other, []error{target}, other},
	} {
		t.Run(tc.caseName, func(t *testing.T) {
			assert.Equal(t, tc.want, Swallow(tc.err, tc.targets...))
		})
	}
}
