package pick

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"
)

// NewMultiSelect returns the checklist model for choosing several entries from
// options under a header and subtext lines, submitLabel on the submit row. No
// options panics.
func NewMultiSelect(header string, subtext []string, submitLabel string, requireSelected bool, options []string) ResponseModel[[]string] {
	return &multiModel{picker: newPicker(header, subtext, submitLabel, requireSelected, options), checked: make([]bool, len(options))}
}

// multiModel is the checklist TUI: the picker chrome plus each option's check
type multiModel struct {
	picker

	checked []bool
}

// Update toggles the cursor's check on enter; the submit row and cancel keys quit
func (m *multiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
func (m *multiModel) submitOn() bool {
	return !m.requireSelected || slices.Contains(m.checked, true)
}

// Response stays non-nil on an empty check set, so callers can tell it from the
// nil non-terminal read
func (m *multiModel) Response() ([]string, error) {
	if m.canceled {
		return nil, ErrCanceled
	}
	picked := make([]string, 0, len(m.options))
	for i, opt := range m.options {
		if m.checked[i] {
			picked = append(picked, opt)
		}
	}
	return picked, nil
}

// View renders the prompt, each line centered in the terminal
func (m *multiModel) View() string {
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
