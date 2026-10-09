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

func TestExit(t *testing.T) {
	tests := []struct {
		name string
		code int
		want error
	}{
		{"Zero", 0, nil},
		{"NonZero", 3, exitError{code: 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Exit(tt.code))
		})
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
		ok   bool
	}{
		{"Nil", nil, 0, false},
		{"Plain", errors.New("nope"), 0, false},
		{"Direct", Exit(3), 3, true},
		{"Wrapped", fmt.Errorf("run: %w", Exit(3)), 3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, ok := ExitCode(tt.err)
			assert.Equal(t, tt.code, code)
			assert.Equal(t, tt.ok, ok)
		})
	}
}
