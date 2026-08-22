// Package cleanup collects teardown steps and runs them LIFO on demand, like defer
// that isn't tied to a function's scope (e.g. handed back to a caller to run later).“
package cleanup

import (
	"slices"

	"github.com/s12chung/ccbox/pkg/util/log"
)

// Stack holds named teardown steps and runs them in reverse order.
type Stack struct {
	steps []func()
}

// Push adds a teardown step; Run logs its error (log.Defer) rather than returning it.
func (s *Stack) Push(what string, fn func() error) {
	s.steps = append(s.steps, func() { log.Defer(what, fn) })
}

// Run runs the pushed steps in reverse (LIFO), the order defer would.
func (s *Stack) Run() {
	for _, step := range slices.Backward(s.steps) {
		step()
	}
}
