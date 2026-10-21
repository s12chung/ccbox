package pick

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// single unwraps the single-pick parts of its picker
func single(t *testing.T, p Picker) *picker[testModel] {
	t.Helper()
	pk, ok := p.(*picker[testModel])
	require.True(t, ok)
	return pk
}

func TestPicker_UpdateEnterSelects(t *testing.T) {
	p := NewPicker(testModels("claude", "codex"))
	s := stack(RunnerOptions{Header: "pick one", SubmitLabel: "Save"}, p)
	s.models[0].cursor = 1

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd, "marking doesn't quit; the submit row does")
	assert.Equal(t, 1, single(t, p).selected)

	var choice testModel
	require.NoError(t, p.respond(&choice))
	assert.Equal(t, testModel{name: "codex"}, choice)
}

func TestPicker_UpdateEnterOnSubmitRow(t *testing.T) {
	p := NewPicker(testModels("claude", "codex"))
	s := stack(RunnerOptions{Header: "pick one", SubmitLabel: "Save"}, p)
	key(t, s, tea.KeyEnter) // marking an item
	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyDown) // onto the submit row

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "submitting quits the program")
	var choice testModel
	require.NoError(t, p.respond(&choice))
	assert.Equal(t, testModel{name: "claude"}, choice)
}

func TestPicker_UpdateSubmitNeedsSelection(t *testing.T) {
	s := stack(RunnerOptions{Header: "pick one", SubmitLabel: "Save", RequireSelected: true}, NewPicker(testModels("claude", "codex")))
	w := s.models[0]
	assert.False(t, s.isSubmitFocusable())

	// nothing marked: the submit row is out of the cursor's path
	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyDown)
	assert.Equal(t, 0, w.cursor)

	key(t, s, tea.KeyEnter) // marking an item arms the submit row
	assert.True(t, s.isSubmitFocusable())

	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyDown)
	assert.Equal(t, -1, s.focus, "the submit row takes the cursor once an item is marked")
}

func TestPicker_Respond(t *testing.T) {
	p := NewPicker(testModels("claude", "codex"))
	p.enter(1)

	var choice testModel
	require.NoError(t, p.respond(&choice))
	assert.Equal(t, testModel{name: "codex"}, choice)

	err := p.respond(new(string))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "want *pick.testModel")

	// nothing marked reads the zero model
	p = NewPicker(testModels("claude", "codex"))
	choice = testModel{}
	require.NoError(t, p.respond(&choice))
	assert.Empty(t, choice.name)
}

func TestPicker_View(t *testing.T) {
	header, subtext := "Select a harness CLI", "(set in ~/.ccbox/config/ccbox.yaml)"
	submitLabel := "Save to ~/.ccbox/config/ccbox.yaml"
	opts := RunnerOptions{Header: header, Subtext: []string{subtext}, SubmitLabel: submitLabel}

	p := NewPicker(testModels("claude", "codex"))
	s := stack(opts, p)
	s.models[0].cursor = 1

	key(t, s, tea.KeyEnter) // mark codex
	assert.Equal(t, []string{
		"  Select a harness CLI",
		"  (set in ~/.ccbox/config/ccbox.yaml)",
		"",
		"    ○ claude",
		"  ❯ ◉ codex",
		"",
		"    Save to ~/.ccbox/config/ccbox.yaml",
		"",
		"  ↑/↓ move • enter select • esc quit",
	}, trimmedLines(s.View()))

	// no subtext: the header flows straight into the blank line before the items
	p = NewPicker(testModels("claude", "codex"))
	assert.Equal(t, []string{
		"  Select a harness CLI",
		"",
		"  ❯ ○ claude",
		"    ○ codex",
		"",
		"    Save to ~/.ccbox/config/ccbox.yaml",
		"",
		"  ↑/↓ move • enter select • esc quit",
	}, trimmedLines(stack(RunnerOptions{Header: header, SubmitLabel: submitLabel}, p).View()))

	// a hinted model carries its gray note parenthesized after its name
	p = NewPicker([]testModel{{name: "claude", hint: "anthropic"}})
	assert.Equal(t, []string{
		"  Select a harness CLI",
		"",
		"  ❯ ○ claude (anthropic)",
		"",
		"    Save to ~/.ccbox/config/ccbox.yaml",
		"",
		"  ↑/↓ move • enter select • esc quit",
	}, trimmedLines(stack(RunnerOptions{Header: header, SubmitLabel: submitLabel}, p).View()))

	// the block is placed in the terminal's size, left-aligned: the run's
	// 2-space padding rides every line's own indent — the rows' 2-space mark
	// column, the submit row's 2
	p = NewPicker(testModels("claude", "codex"))
	s = stack(opts, p)
	s.models[0].cursor = 1
	key(t, s, tea.KeyEnter) // mark codex
	s.width, s.height = 40, 10
	view := s.View()
	wantLeading := map[string]int{
		"Select a harness CLI":                2,
		"(set in ~/.ccbox/config/ccbox.yaml)": 2,
		"○ claude":                            4,
		"❯ ◉ codex":                           2,
		"Save to ~/.ccbox/config/ccbox.yaml":  4,
		"↑/↓ move • enter select • esc quit":  2,
	}
	for line := range strings.SplitSeq(strings.TrimSuffix(view, "\n"), "\n") {
		assert.Equal(t, 40, lipgloss.Width(line), "each line spans the placed width")
		line = strings.TrimRight(line, " ")
		if line == "" {
			continue
		}
		content := strings.TrimLeft(line, " ")
		if assert.Contains(t, wantLeading, content) {
			assert.Equal(t, wantLeading[content], lipgloss.Width(line)-lipgloss.Width(content), "line %q is left-aligned", content)
			delete(wantLeading, content)
		}
	}
	assert.Empty(t, wantLeading, "every placed line was checked")
}

func TestNewPicker_NoItems(t *testing.T) {
	assert.Panics(t, func() { NewPicker[testModel](nil) })
	assert.Panics(t, func() { NewPicker([]testModel{}) })
}
