// Package pick shows bubbletea prompts for choosing: one entry from options,
// several entries, or yes/no.
package pick

import (
	"errors"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/moby/term"
)

// ErrCanceled is returned when the user quits the prompt instead of choosing (esc/ctrl+c).
var ErrCanceled = errors.New("selection canceled")

// the prompt's palette: white header/cursor/marked option, slate options, gray help
var (
	whiteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	slateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7599B4"))
	grayStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#616161"))
)

// NewSelect returns the single-pick model for choosing one entry from options
// under a header and subtext lines, submitLabel on the submit row. No options
// panics.
func NewSelect(header string, subtext []string, submitLabel string, requireSelected bool, options []string) ResponseModel[string] {
	return &model{picker: newPicker(header, subtext, submitLabel, requireSelected, options), selected: -1}
}

// ResponseModel is either picker TUI, ready to Run
type ResponseModel[A any] interface {
	tea.Model

	// Response returns the prompt's read; quitting the prompt reads ErrCanceled
	Response() (A, error)
}

// Run shows m's TUI on the alt screen. A non-terminal stdin/stdout is a no-op.
func Run(m tea.Model) error {
	if !interactive() {
		return nil
	}
	_, err := tea.NewProgram(m, tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout), tea.WithAltScreen()).Run()
	return err
}

// RunWithResponse asks m's prompt: Run, then Response. The non-terminal read is
// the zero answer; quitting aborts with ErrCanceled.
func RunWithResponse[A any](m ResponseModel[A]) (A, error) {
	if !interactive() {
		var zero A
		return zero, nil
	}
	if err := Run(m); err != nil {
		var zero A
		return zero, err
	}
	return m.Response()
}

func interactive() bool { return isTerminal(os.Stdin) && isTerminal(os.Stdout) }

func isTerminal(f *os.File) bool {
	fd, _ := term.GetFdInfo(f)
	return term.IsTerminal(fd)
}

// picker is the chrome both pickers share: a header, subtext lines, a cursor over
// the options plus a final submit row, and a help line, all centered in the terminal
type picker struct {
	header          string
	subtext         []string
	options         []string
	submitLabel     string // the final row's label; enter on it confirms the prompt
	requireSelected bool   // the submit row stays gray and out of the cursor's path until a selection
	cursor          int
	canceled        bool
	width           int
	height          int
}

func newPicker(header string, subtext []string, submitLabel string, requireSelected bool, options []string) picker {
	if len(options) == 0 {
		panic("pick: no options to pick from")
	}
	return picker{header: header, subtext: subtext, options: options, submitLabel: submitLabel, requireSelected: requireSelected}
}

func (p picker) Init() tea.Cmd { return nil }

// key handles the keys both pickers share: moving the cursor and quitting on
// esc/ctrl+c. The submit row joins the cursor's path only when submitOn. quit
// reports the prompt should close.
func (p picker) key(msg tea.KeyMsg, submitOn bool) (picker, bool) {
	rows := len(p.options)
	if submitOn {
		rows++ // the submit row
	}
	switch msg.String() {
	case "up", "k":
		p.cursor = (p.cursor - 1 + rows) % rows
	case "down", "j":
		p.cursor = (p.cursor + 1) % rows
	case "esc", "ctrl+c":
		p.canceled = true
		return p, true
	}
	return p, false
}

// submitRow renders the final row: enter on it confirms the prompt. A disabled
// submit is gray — nothing chosen yet, so it's out of the cursor's path too
func (p picker) submitRow(enabled bool) string {
	if !enabled {
		return "  " + grayStyle.Render(p.submitLabel)
	}
	if p.cursor == len(p.options) {
		return whiteStyle.Render("❯ " + p.submitLabel)
	}
	return "  " + slateStyle.Render(p.submitLabel)
}

// place centers the prompt in the terminal, each line centered on its own: the
// header, the subtext, then the body lines
func (p picker) place(body []string) string {
	lines := make([]string, 0, 1+len(p.subtext)+len(body))
	lines = append(lines, whiteStyle.Render(p.header))
	// the subtext renders per line: a style render pads a multi-line string to one
	// block width, which Place would then center as a unit instead of centering each line
	for _, line := range p.subtext {
		lines = append(lines, grayStyle.Render(line))
	}
	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, strings.Join(append(lines, body...), "\n"))
}

// model is the single-pick TUI: the picker chrome plus the option marked with
// the circle
type model struct {
	picker

	selected int // -1 is nothing marked yet
}

func (m *model) Response() (string, error) {
	if m.canceled {
		return "", ErrCanceled
	}
	if m.selected < 0 {
		return "", nil
	}
	return m.options[m.selected], nil
}

// submitOn reports the submit row takes the cursor and enter
func (m *model) submitOn() bool { return !m.requireSelected || m.selected >= 0 }

// Update marks the cursor's option on enter; the submit row and cancel keys quit
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			m.selected = m.cursor
		}
	}
	return m, nil
}

// View renders the prompt, each line centered in the terminal
func (m *model) View() string {
	body := []string{""}
	for i, opt := range m.options {
		circle, style := "○", slateStyle
		if i == m.selected {
			circle, style = "◉", whiteStyle
		}
		cursor := "  "
		if i == m.cursor {
			cursor = whiteStyle.Render("❯ ")
		}
		body = append(body, cursor+circle+" "+style.Render(opt))
	}
	body = append(body, "", m.submitRow(m.submitOn()))
	return m.place(append(body, "", grayStyle.Render("↑/↓ move • enter select • esc quit")))
}
