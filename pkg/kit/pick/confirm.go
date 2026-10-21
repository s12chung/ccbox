package pick

import (
	"errors"
)

// yesNo is Confirm's two answers as Models — the bool rides the pick
type yesNo bool

func (y yesNo) Name() string {
	if y {
		return "Yes"
	}
	return "No"
}
func (y yesNo) Hint() string { return "" }

// Confirm asks a yes/no question as a TUI list of the two answers, returning
// true only on "Yes". Quitting the prompt answers no too.
func Confirm(question string) (bool, error) {
	var answer yesNo
	err := NewRunner(RunnerOptions{Header: question, SubmitLabel: "Confirm"},
		NewPicker([]yesNo{false, true})).Run(&answer)
	if err != nil && !errors.Is(err, ErrCanceled) {
		return false, err
	}
	return bool(answer), nil
}
