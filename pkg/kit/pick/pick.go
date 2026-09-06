// Package pick shows bubbletea prompts for choosing: one entry from options,
// or yes/no.
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

// the prompt's palette: white header/cursor/choice, slate options, gray help
var (
	whiteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	slateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7599B4"))
	grayStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#616161"))
)

// Select runs a TUI list of options under a header and subtext lines, returning the
// chosen entry. The list paints on the alt screen, so nothing is left in stdout when
// it closes. A non-terminal stdin/stdout can't show the prompt, so it returns "" with
// a nil error — callers keep their default. Quitting the prompt aborts with ErrCanceled.
func Select(header string, subtext []string, options []string) (string, error) {
	if !isTerminal(os.Stdin) || !isTerminal(os.Stdout) || len(options) == 0 {
		return "", nil
	}
	final, err := tea.NewProgram(newModel(header, subtext, options), tea.WithInput(os.Stdin), tea.WithOutput(os.Stdout), tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	m, ok := final.(model)
	if !ok {
		return "", errors.New("pick: unexpected program result")
	}
	if m.canceled {
		return "", ErrCanceled
	}
	return m.choice, nil
}

func isTerminal(f *os.File) bool {
	fd, _ := term.GetFdInfo(f)
	return term.IsTerminal(fd)
}

// model is the TUI list: a header and subtext lines, a cursor among options, and a
// help line, all centered in the terminal
type model struct {
	header   string
	subtext  []string
	options  []string
	cursor   int
	choice   string
	canceled bool
	width    int
	height   int
}

func newModel(header string, subtext []string, options []string) model {
	return model{header: header, subtext: subtext, options: options}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			m.cursor = (m.cursor - 1 + len(m.options)) % len(m.options)
		case "down", "j":
			m.cursor = (m.cursor + 1) % len(m.options)
		case "enter":
			m.choice = m.options[m.cursor]
			return m, tea.Quit
		case "esc", "ctrl+c":
			m.canceled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString(whiteStyle.Render(m.header) + "\n")
	// rendered per line: a style render pads a multi-line string to one block width,
	// which Place would then center as a unit instead of centering each line
	for _, line := range m.subtext {
		b.WriteString(grayStyle.Render(line) + "\n")
	}
	b.WriteString("\n")
	for i, opt := range m.options {
		if i == m.cursor {
			b.WriteString(whiteStyle.Render("❯ "+opt) + "\n")
			continue
		}
		b.WriteString("  " + slateStyle.Render(opt) + "\n")
	}
	b.WriteString("\n" + grayStyle.Render("↑/↓ move • enter select • esc quit"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, b.String())
}
