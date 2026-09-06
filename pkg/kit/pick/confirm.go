package pick

import (
	"errors"
)

// Confirm asks a yes/no question as a TUI list of the two answers, returning
// true only on "Yes". Quitting the prompt answers no too.
func Confirm(question string) (bool, error) {
	answer, err := Select(question, nil, []string{"No", "Yes"})
	if err != nil && !errors.Is(err, ErrCanceled) {
		return false, err
	}
	return answer == "Yes", nil
}
