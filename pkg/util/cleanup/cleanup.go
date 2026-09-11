// Package cleanup collects teardown steps and runs them LIFO on demand, like defer
// that isn't tied to a function's scope (e.g. handed back to a caller to run later).
package cleanup

import (
	"errors"
	"fmt"
	"slices"
)

// step is one named teardown.
type step struct {
	what string
	fn   func() error
}

// Stack holds named teardown steps and runs them in reverse order.
type Stack struct {
	steps []step
}

// Push adds a teardown step; a nil fn is no step.
func (s *Stack) Push(what string, fn func() error) {
	if fn == nil {
		return
	}
	s.steps = append(s.steps, step{what: what, fn: fn})
}

// Run runs the pushed steps in reverse (LIFO), the order defer would.
func (s *Stack) Run() error {
	var errs []error
	for _, st := range slices.Backward(s.steps) {
		if err := st.fn(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", st.what, err))
		}
	}
	return errors.Join(errs...)
}
