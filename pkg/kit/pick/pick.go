// Package pick provides a composable framework separating the bubbletea
// chrome from stackable Pickers.
package pick

import (
	"errors"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/moby/term"
)

// ErrCanceled is returned when the user quits the prompt instead of choosing (esc/ctrl+c).
var ErrCanceled = errors.New("selection canceled")

var (
	defaultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7599B4")) // slate
	strongStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")) // white
	lightStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#616161")) // gray
)

// RunnerOptions is the chrome a Runner renders: a header, subtext lines, and
// the label of the run's one submit row
type RunnerOptions struct {
	Header      string
	Subtext     []string
	SubmitLabel string

	// the submit row stays gray and out of the cursor's path until every
	// picker has its pick
	RequireSelected bool
}

// Runner is the prompt chrome over the stacked pickers, closed by the one
// submit row
type Runner struct {
	opts    RunnerOptions
	pickers []*pickerModel
}

// NewRunner is the chrome over the pickers, asked in order
func NewRunner(opts RunnerOptions, pickers ...Picker) Runner {
	r := Runner{opts: opts, pickers: make([]*pickerModel, len(pickers))}
	for i, pk := range pickers {
		r.pickers[i] = &pickerModel{Picker: pk}
	}
	return r
}

// Run asks the stacked pickers in one TUI read, filling each pointer with its
// picker's read in the Runner's order. The non-terminal read is the zero
// answers; quitting aborts with ErrCanceled.
func (r Runner) Run(ptrs ...any) error {
	if len(ptrs) != len(r.pickers) {
		return fmt.Errorf("pick: response pointer count (%d) does not match picker count (%d)", len(ptrs), len(r.pickers))
	}
	if len(r.pickers) == 0 {
		return errors.New("pick: no pickers to ask")
	}
	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) { // check if interactive terminal
		return nil
	}
	s := &stackModel{opts: r.opts, models: r.pickers, done: make(chan struct{}, 1)}
	p := tea.NewProgram(s, tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	select {
	case <-s.done:
		return ErrCanceled
	default: // a run closed without canceling — a gone terminal — reads submitted
	}
	for i, pm := range r.pickers {
		if err := pm.respond(ptrs[i]); err != nil { // #nosec G602 -- the counts were checked equal above
			return err
		}
	}
	return nil
}

// Picker is one question the Runner asks: its rows under the cursor, what
// enter does on the cursor's row, whether it has its pick, and the read it
// writes into the caller's pointer
type Picker interface {
	// rows counts the body's rows — the cursor's bounds ride the count: rows
	// can exceed the options with revealed rows
	rows() int

	// render draws the body's rows — the cursor's mark rides the focused prompt
	render(cursor int, focused bool) []string

	// enter answers enter on the cursor's row
	enter(cursor int)

	// picked reports the picker has a pick: the single picker's mark, the
	// checklist's checks
	picked() bool

	// respond writes the picker's read into the caller's pointer
	respond(ptr any) error
}

// pickerModel is a picker as the Runner renders it: the picker's rows under
// the run's cursor
type pickerModel struct {
	Picker

	cursor int
}

// stackModel is the one model a Runner shows: its pickers stacked under the
// chrome, closed by the one submit row
type stackModel struct {
	opts   RunnerOptions
	models []*pickerModel
	focus  int           // the picker taking the cursor; -1 is the submit row
	done   chan struct{} // carries a cancel back to Run; cap 1 so the send never blocks
	width  int
	height int
}

func (s *stackModel) Init() tea.Cmd { return nil }

// Update sizes the placed view, answers the keys through key, and quits on quitMsg
func (s *stackModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
	case quitMsg:
		return s, tea.Quit
	case tea.KeyMsg:
		if cmd := s.key(msg); cmd != nil {
			return s, cmd
		}
	}
	return s, nil
}

// key handles the run's keys; its cmd closes the run, nil keeps it open
func (s *stackModel) key(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "ctrl+c":
		return s.cancel()
	case "up", "k":
		s.move(-1)
	case "down", "j":
		s.move(1)
	case "enter":
		if s.isSubmitFocused() {
			return tea.Quit
		}
		m := s.models[s.focus]
		m.enter(m.cursor)
	}
	return nil
}

// quitMsg closes the run once its cancel reached Run
type quitMsg struct{}

// cancel quits the run as canceled, the cancel riding the cmd back to Run
func (s *stackModel) cancel() tea.Cmd {
	return func() tea.Msg {
		s.done <- struct{}{}
		return quitMsg{}
	}
}

// move steps the cursor one row in step's direction — within the focused
// picker, or off its edge onto the run's next slot. The submit row's focus
// -1 needs no case of its own: cross's wrap lands it right
func (s *stackModel) move(step int) {
	if !s.isSubmitFocused() {
		m := s.models[s.focus]
		if next := m.cursor + step; next >= 0 && next < m.rows() {
			m.cursor = next
			return
		}
	}
	s.cross(s.focus+step, step)
}

// cross lands the cursor on the run's slot i
func (s *stackModel) cross(i int, step int) {
	if slots := len(s.models) + 1; i < 0 || i >= slots { // +1 for the submit row
		i = (i + slots) % slots
	}
	if i == len(s.models) { // the submit row
		if s.isSubmitFocusable() {
			s.focus = -1
			return
		}
		s.cross(i+step, step) // a gray submit row: recurse to the slot beyond
		return
	}
	s.focus = i
	if m := s.models[i]; step > 0 {
		m.cursor = 0
	} else {
		m.cursor = m.rows() - 1
	}
}

// isSubmitFocusable reports the submit row takes the cursor and enter — every picker
// has its pick, unless RequireSelected is off
func (s *stackModel) isSubmitFocusable() bool {
	if !s.opts.RequireSelected {
		return true
	}
	for _, m := range s.models {
		if !m.picked() {
			return false
		}
	}
	return true
}

// isSubmitFocused reports the cursor sits on the run's one submit row
func (s *stackModel) isSubmitFocused() bool { return s.focus < 0 }

// submitRow renders the run's one submit row; a disabled one is gray
func (s *stackModel) submitRow() string {
	if !s.isSubmitFocusable() {
		return "  " + lightStyle.Render(s.opts.SubmitLabel)
	}
	if s.isSubmitFocused() {
		return strongStyle.Render("❯ " + s.opts.SubmitLabel)
	}
	return "  " + defaultStyle.Render(s.opts.SubmitLabel)
}

// View renders the pickers under the chrome — the header and subtext over
// them, the submit and help rows below — all centered in the terminal
func (s *stackModel) View() string {
	lines := make([]string, 0, 2+len(s.opts.Subtext))
	lines = append(lines, strongStyle.Render(s.opts.Header))
	// render each line separately: a multi-line render pads its lines to one
	// width, so Place would center the block instead of each line
	for _, line := range s.opts.Subtext {
		lines = append(lines, lightStyle.Render(line))
	}
	lines = append(lines, "")
	for i, m := range s.models {
		if i > 0 { // a blank line separates the stacked pickers
			lines = append(lines, "")
		}
		lines = append(lines, m.render(m.cursor, i == s.focus)...)
	}
	body := strings.Join(lines, "\n") + "\n\n" + s.submitRow() + "\n\n" +
		lightStyle.Render("↑/↓ move • enter select • esc quit")
	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, body)
}

func isTerminal(f *os.File) bool {
	fd, _ := term.GetFdInfo(f)
	return term.IsTerminal(fd)
}

// fill writes a picker's read into the caller's pointer
func fill[A any](ptr any, a A) error {
	p, ok := ptr.(*A)
	if !ok {
		return fmt.Errorf("pick: response pointer is %T, want %T", ptr, p)
	}
	*p = a
	return nil
}
