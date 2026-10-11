package pick

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"
)

// MultiSelect runs a TUI checklist under a header and subtext lines, returning
// the checked entries in the options' order. The headless read is nil, so an
// empty check set returns an empty, non-nil slice. Quitting aborts with ErrCanceled.
func MultiSelect(header string, subtext []string, submitLabel string, requireSelected bool, options []string) ([]string, error) {
	if noprompt(options) {
		return nil, nil
	}
	m, err := run(multiModel{picker: newPicker(header, subtext, submitLabel, requireSelected, options), checked: make([]bool, len(options))})
	if err != nil {
		return nil, err
	}
	if m.canceled {
		return nil, ErrCanceled
	}
	return m.picked(), nil
}

// multiModel is the checklist TUI: the picker chrome plus each option's check
type multiModel struct {
	picker

	checked []bool
}

func (m multiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		p, quit := m.key(msg, m.submitOn())
		m.picker = p
		if quit {
			return m, tea.Quit
		}
		if msg.String() == "enter" {
			if m.cursor == len(m.options) { // the submit row
				return m, tea.Quit
			}
			m.checked[m.cursor] = !m.checked[m.cursor]
		}
	}
	return m, nil
}

// submitOn reports the submit row takes the cursor and enter
func (m multiModel) submitOn() bool {
	return !m.requireSelected || slices.Contains(m.checked, true)
}

// picked stays non-nil on an empty check set, so callers can tell it from the
// nil no-terminal read
func (m multiModel) picked() []string {
	picked := make([]string, 0, len(m.options))
	for i, opt := range m.options {
		if m.checked[i] {
			picked = append(picked, opt)
		}
	}
	return picked
}

func (m multiModel) View() string {
	body := []string{""}
	for i, opt := range m.options {
		box, style := "[ ]", slateStyle
		if m.checked[i] {
			box, style = "[x]", whiteStyle
		}
		cursor := "  "
		if i == m.cursor {
			cursor = whiteStyle.Render("❯ ")
		}
		body = append(body, cursor+box+" "+style.Render(opt))
	}
	body = append(body, "", m.submitRow(m.submitOn()))
	return m.place(append(body, "", grayStyle.Render("↑/↓ move • enter check • esc quit")))
}
