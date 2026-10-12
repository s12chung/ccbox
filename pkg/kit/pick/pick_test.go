package pick

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newModel builds the single-pick model for the tests; NewSelect inlines the literal
func newModel(header string, subtext []string, submitLabel string, requireSelected bool, options []string) *model {
	return &model{picker: newPicker(header, subtext, submitLabel, requireSelected, options), selected: -1}
}

// key sends a key press to m and returns the updated model
func key[T tea.Model](t *testing.T, m T, k tea.KeyType) T {
	t.Helper()
	next, _ := m.Update(tea.KeyMsg{Type: k})
	updated, ok := next.(T)
	require.True(t, ok)
	return updated
}

func TestModel_UpdateMovesCursor(t *testing.T) {
	m := newModel("pick one", nil, "Save", false, []string{"claude", "codex", "grok"})
	assert.Equal(t, 0, m.cursor)

	m = key(t, m, tea.KeyDown)
	assert.Equal(t, 1, m.cursor)
	m = key(t, m, tea.KeyUp)
	assert.Equal(t, 0, m.cursor)

	m = key(t, m, tea.KeyUp)
	assert.Equal(t, 3, m.cursor, "wraps up onto the submit row")

	m = newModel("pick one", nil, "Save", false, []string{"claude", "codex", "grok"})
	for range 4 {
		m = key(t, m, tea.KeyDown)
	}
	assert.Equal(t, 0, m.cursor, "wraps down past the submit row")
}

func TestModel_UpdateEnterSelects(t *testing.T) {
	m := newModel("pick one", nil, "Save", false, []string{"claude", "codex"})
	m.cursor = 1

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, ok := next.(*model)
	require.True(t, ok)
	assert.Equal(t, 1, updated.selected)
	assert.Nil(t, cmd, "marking doesn't quit; the submit row does")
}

func TestModel_UpdateEnterOnSubmitRow(t *testing.T) {
	m := newModel("pick one", nil, "Save", false, []string{"claude", "codex"})
	m.cursor = len(m.options) // the submit row
	m.selected = 1

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, ok := next.(*model)
	require.True(t, ok)
	assert.Equal(t, 1, updated.selected)
	require.NotNil(t, cmd, "submitting quits the program")
}

func TestModel_UpdateSubmitNeedsSelection(t *testing.T) {
	m := newModel("pick one", nil, "Save", true, []string{"claude", "codex"})
	assert.False(t, m.submitOn())

	// nothing marked: the gray submitLabel row is out of the cursor's path
	m = key(t, key(t, m, tea.KeyDown), tea.KeyDown)
	assert.Equal(t, 0, m.cursor)

	m = key(t, m, tea.KeyEnter) // marking an option arms the submit row
	assert.Equal(t, 0, m.selected)
	assert.True(t, m.submitOn())

	m = key(t, key(t, m, tea.KeyDown), tea.KeyDown)
	assert.Equal(t, 2, m.cursor, "the submit row takes the cursor once an option is marked")

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, ok := next.(*model)
	require.True(t, ok)
	require.NotNil(t, cmd, "submitting quits the program")
	assert.Equal(t, 0, updated.selected)
}

func TestModel_UpdateCancelKeys(t *testing.T) {
	for _, k := range []tea.KeyType{tea.KeyEsc, tea.KeyCtrlC} {
		m := key(t, newModel("pick one", nil, "Save", false, []string{"claude"}), k)
		assert.True(t, m.canceled, k)
		assert.Equal(t, -1, m.selected, k)

		_, err := m.Response()
		assert.ErrorIs(t, err, ErrCanceled, k)
	}
}

func TestModel_UpdateTracksWindowSize(t *testing.T) {
	m := newModel("pick one", nil, "Save", false, []string{"claude"})
	next, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	updated, ok := next.(*model)
	require.True(t, ok)
	assert.Equal(t, 100, updated.width)
	assert.Equal(t, 30, updated.height)
	assert.Nil(t, cmd)
}

func TestModel_View(t *testing.T) {
	header, subtext := "Select a harness CLI", "(set in ~/.ccbox/config/ccbox.yaml)"
	submitLabel := "Save to ~/.ccbox/config/ccbox.yaml"

	m := newModel(header, []string{subtext}, submitLabel, false, []string{"claude", "codex"})
	m.cursor, m.selected = 1, 1
	assert.Contains(t, m.View(), header)
	assert.Contains(t, m.View(),
		"Select a harness CLI\n(set in ~/.ccbox/config/ccbox.yaml)\n\n  ○ claude\n❯ ◉ codex\n\n  Save to ~/.ccbox/config/ccbox.yaml\n")

	// no subtext: the header flows straight into the blank line before the options
	m = newModel(header, nil, submitLabel, false, []string{"claude", "codex"})
	assert.Contains(t, m.View(), "Select a harness CLI\n\n❯ ○ claude\n")

	// the block is placed in the terminal's size, each line centered on its own:
	// floor((40-w)/2) left padding, plus each option's own 2-space indent
	m = newModel(header, []string{subtext}, submitLabel, false, []string{"claude", "codex"})
	m.cursor, m.selected = 1, 1
	m.width, m.height = 40, 10
	view := m.View()
	wantLeading := map[string]int{
		"Select a harness CLI":                10,
		"(set in ~/.ccbox/config/ccbox.yaml)": 2,
		"○ claude":                            17,
		"❯ ◉ codex":                           15,
		"Save to ~/.ccbox/config/ccbox.yaml":  4,
		"↑/↓ move • enter select • esc quit":  3,
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

func TestNewSelect_NoOptions(t *testing.T) {
	assert.Panics(t, func() { NewSelect("pick one", nil, "Save", false, nil) })
	assert.Panics(t, func() { NewSelect("pick one", nil, "Save", false, []string{}) })
}

func TestRunHeadless(t *testing.T) {
	// go test pipes stdin/stdout: no terminal, no prompt, no error
	assert.NoError(t, Run(newModel("pick one", nil, "Save", false, []string{"claude"})))
}

func TestSelectHeadless(t *testing.T) {
	// go test pipes stdin/stdout: no terminal, no prompt, no error
	choice, err := RunWithResponse(NewSelect("pick one", nil, "Save", false, []string{"claude"}))
	require.NoError(t, err)
	assert.Empty(t, choice)
}

func TestConfirmHeadless(t *testing.T) {
	// go test pipes stdin/stdout: no terminal, no prompt, no is the safe default
	ok, err := Confirm("reseed?")
	require.NoError(t, err)
	assert.False(t, ok)
}
