package pick

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// key sends a key press to m and returns the updated model
func key(t *testing.T, m model, k tea.KeyType) model {
	t.Helper()
	next, _ := m.Update(tea.KeyMsg{Type: k})
	updated, ok := next.(model)
	require.True(t, ok)
	return updated
}

func TestModel_UpdateMovesCursor(t *testing.T) {
	m := newModel("pick one", nil, []string{"claude", "codex", "grok"})
	assert.Equal(t, 0, m.cursor)

	m = key(t, m, tea.KeyDown)
	assert.Equal(t, 1, m.cursor)
	m = key(t, m, tea.KeyUp)
	assert.Equal(t, 0, m.cursor)

	assert.Equal(t, 2, key(t, m, tea.KeyUp).cursor, "wraps up past the top")
	assert.Equal(t, 2, key(t, key(t, m, tea.KeyDown), tea.KeyDown).cursor, "wraps down past the bottom")
}

func TestModel_UpdateEnterChooses(t *testing.T) {
	m := newModel("pick one", nil, []string{"claude", "codex"})
	m.cursor = 1

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, ok := next.(model)
	require.True(t, ok)
	assert.Equal(t, "codex", updated.choice)
	assert.False(t, updated.canceled)
	require.NotNil(t, cmd, "enter quits the program")
}

func TestModel_UpdateCancelKeys(t *testing.T) {
	for _, k := range []tea.KeyType{tea.KeyEsc, tea.KeyCtrlC} {
		m := key(t, newModel("pick one", nil, []string{"claude"}), k)
		assert.True(t, m.canceled, k)
		assert.Empty(t, m.choice, k)
	}
}

func TestModel_UpdateTracksWindowSize(t *testing.T) {
	m := newModel("pick one", nil, []string{"claude"})
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	updated, ok := next.(model)
	require.True(t, ok)
	assert.Equal(t, 100, updated.width)
	assert.Equal(t, 30, updated.height)
	assert.Nil(t, cmd)
}

func TestModel_View(t *testing.T) {
	m := newModel("Select a harness CLI", []string{"(set in ~/.ccbox/config/ccbox.yaml)"}, []string{"claude", "codex"})
	m.cursor = 1
	assert.Contains(t, m.View(), "Select a harness CLI")
	assert.Contains(t, m.View(), "Select a harness CLI\n(set in ~/.ccbox/config/ccbox.yaml)\n\n  claude\n❯ codex\n")

	// no subtext: the header flows straight into the blank line before the options
	m = newModel("Select a harness CLI", nil, []string{"claude", "codex"})
	assert.Contains(t, m.View(), "Select a harness CLI\n\n❯ claude\n")

	// the block is placed in the terminal's size, each line centered on its own:
	// floor((40-w)/2) left padding, plus each option's own 2-space indent
	m = newModel("Select a harness CLI", []string{"(set in ~/.ccbox/config/ccbox.yaml)"}, []string{"claude", "codex"})
	m.cursor = 1
	m.width, m.height = 40, 10
	view := m.View()
	wantLeading := map[string]int{
		"Select a harness CLI":                10,
		"(set in ~/.ccbox/config/ccbox.yaml)": 2,
		"claude":                              18, // 16 place + 2 option indent
		"❯ codex":                             16,
		"↑/↓ move • enter select • esc quit": 3,
	}
	for line := range strings.SplitSeq(strings.TrimSuffix(view, "\n"), "\n") {
		assert.Equal(t, 40, lipgloss.Width(line), "each line spans the placed width")
		line = strings.TrimRight(line, " ")
		if line == "" {
			continue
		}
		content := strings.TrimLeft(line, " ")
		if assert.Contains(t, wantLeading, content) {
			assert.Equal(t, wantLeading[content], lipgloss.Width(line)-lipgloss.Width(content), "line %q is centered", content)
			delete(wantLeading, content)
		}
	}
	assert.Empty(t, wantLeading, "every placed line was checked")
}

func TestSelectHeadless(t *testing.T) {
	// go test pipes stdin/stdout: no terminal, no prompt, no error
	choice, err := Select("pick one", nil, []string{"claude"})
	require.NoError(t, err)
	assert.Empty(t, choice)
}

func TestConfirmHeadless(t *testing.T) {
	// go test pipes stdin/stdout: no terminal, no prompt, no is the safe default
	ok, err := Confirm("reseed?")
	require.NoError(t, err)
	assert.False(t, ok)
}
