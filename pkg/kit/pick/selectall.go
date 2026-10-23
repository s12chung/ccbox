package pick

import "errors"

// SelectAllPicker is the checklist parts the select-all row drives: the mass
// check, and the read its own check renders from
type SelectAllPicker interface {
	Picker

	SelectAll()
	AllSelected() bool
}

// SelectAll is the one Select All row heading the checklist pickers it drives
type SelectAll struct {
	Pickers []SelectAllPicker
}

// PickerStack returns the pickers for one Runner ask: the row heading its pickers
func (s SelectAll) PickerStack() []Picker {
	stack := make([]Picker, 0, len(s.Pickers)+1)
	stack = append(stack, s)
	for _, picker := range s.Pickers {
		stack = append(stack, picker)
	}
	return stack
}

func (s SelectAll) rows() int { return 1 }

// render draws the row, its check read off the pickers
func (s SelectAll) render(cursor int, focused bool) []string {
	mark := "  "
	if focused && cursor == 0 {
		mark = strongStyle.Render("❯ ")
	}
	box := defaultStyle.Render("[ ]")
	if s.allSelected() {
		box = strongStyle.Render("[x]")
	}
	return []string{mark + box + " Select All"}
}

// enter checks every row of every picker
func (s SelectAll) enter(int) {
	for _, p := range s.Pickers {
		p.SelectAll()
	}
}

// picked reports the row never holds the submit back — its answer is the
// pickers'
func (s SelectAll) picked() bool { return true }

// respond reads nothing — the row's answer is the pickers'; the caller's
// pointer for it is nil
func (s SelectAll) respond(ptr any) error {
	if ptr == nil {
		return nil
	}
	return errors.New("pick: the select-all row reads nothing")
}

// allSelected reports the pickers non-empty and every one of them has all its
// rows checked — nothing to select is not all selected
func (s SelectAll) allSelected() bool {
	if len(s.Pickers) == 0 {
		return false
	}
	for _, p := range s.Pickers {
		if !p.AllSelected() {
			return false
		}
	}
	return true
}
