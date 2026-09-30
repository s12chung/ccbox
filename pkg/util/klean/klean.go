// Package klean collects teardown steps and runs them on demand, like defer that
// isn't tied to a function's scope (e.g. handed back to a caller to run later):
// Stack runs its steps in reverse, all of them; Chain runs them in order, stopping
// at the first error.
package klean

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

// Chain holds steps run in order — unlike Stack's LIFO run-all, later steps run only
// while earlier ones succeed: the first error stops the chain and returns.
type Chain struct {
	steps []func() error
}

// NewChain returns a Chain of fns, in order; a nil fn is no step.
func NewChain(fns ...func() error) *Chain {
	var c Chain
	for _, fn := range fns {
		if fn != nil {
			c.steps = append(c.steps, fn)
		}
	}
	return &c
}

// Run runs the pushed steps in order, stopping at and returning the first error.
func (c *Chain) Run() error {
	for _, fn := range c.steps {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

// SwallowErr returns a func running fn and swallowing err: the token error a step
// reports to stop its successors without failing the run.
func SwallowErr(fn func() error, err error) func() error {
	return func() error {
		if runErr := fn(); !errors.Is(runErr, err) {
			return runErr
		}
		return nil
	}
}
