package pick

import (
	"errors"
)

// Confirm asks a yes/no question as a TUI list of the two answers, returning
// true only on "Yes". Quitting the prompt answers no too.
func Confirm(question string) (bool, error) {
	var answer string
	err := NewRunner(RunnerOptions{Header: question, SubmitLabel: "Confirm"},
		NewPicker([]string{"No", "Yes"})).Run(&answer)
	if err != nil && !errors.Is(err, ErrCanceled) {
		return false, err
	}
	return answer == "Yes", nil
}
