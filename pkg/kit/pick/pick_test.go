package pick

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunner_RunHeadless(t *testing.T) {
	// go test pipes stdin/stdout: no terminal, no prompt, no error, the zero read
	var choice string
	require.NoError(t, NewRunner(RunnerOptions{Header: "pick one", SubmitLabel: "Save"}, NewPicker([]string{"claude"})).Run(&choice))
	assert.Empty(t, choice)
}

func TestRunner_RunPtrCount(t *testing.T) {
	err := NewRunner(RunnerOptions{Header: "pick one", SubmitLabel: "Save"}, NewPicker([]string{"claude"})).Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match picker count (1)")
}

func TestRunner_RunNoPrompts(t *testing.T) {
	require.Error(t, Runner{}.Run())
}

func key(t *testing.T, s *stackModel, k tea.KeyType) {
	t.Helper()
	s.Update(tea.KeyMsg{Type: k})
}

// stack stacks the pickers under the chrome like a Runner, the top one taking the cursor
func stack(opts RunnerOptions, pickers ...Picker) *stackModel {
	r := NewRunner(opts, pickers...)
	return &stackModel{opts: r.opts, models: r.pickers, done: make(chan struct{}, 1)}
}

func TestStackModel_UpdateMovesCursor(t *testing.T) {
	s := stack(RunnerOptions{Header: "pick one", SubmitLabel: "Save"}, NewPicker([]string{"claude", "codex", "grok"}))
	w := s.models[0]
	assert.Equal(t, 0, w.cursor)

	key(t, s, tea.KeyDown)
	assert.Equal(t, 1, w.cursor)
	key(t, s, tea.KeyUp)
	assert.Equal(t, 0, w.cursor)

	key(t, s, tea.KeyUp)
	assert.Equal(t, -1, s.focus, "wraps up onto the submit row")

	key(t, s, tea.KeyDown)
	assert.Equal(t, 0, s.focus, "wraps down off the submit row")
	assert.Equal(t, 0, w.cursor)

	s = stack(RunnerOptions{Header: "pick one", SubmitLabel: "Save"}, NewPicker([]string{"claude", "codex", "grok"}))
	w = s.models[0]
	for range 4 {
		key(t, s, tea.KeyDown)
	}
	assert.Equal(t, 0, w.cursor, "wraps down past the submit row")
}

func TestStackModel_UpdateMovesAcrossModels(t *testing.T) {
	p1 := NewPicker([]string{"claude", "codex"})
	p2 := NewPicker([]string{"pi", "gemini"})
	s := stack(RunnerOptions{Header: "pick two", SubmitLabel: "save"}, p1, p2)
	w1, w2 := s.models[0], s.models[1]

	// down past p1's last row crosses into p2's first
	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyDown)
	assert.Equal(t, 1, s.focus)
	assert.Equal(t, 0, w2.cursor)

	// up back off p2's first row lands on p1's last
	key(t, s, tea.KeyUp)
	assert.Equal(t, 0, s.focus)
	assert.Equal(t, 1, w1.cursor)

	// down onto p2's last, once more: the submit row takes the cursor
	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyDown)
	assert.Equal(t, -1, s.focus)

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "submitting quits the program")
}

func TestStackModel_UpdateMovesOverGraySubmit(t *testing.T) {
	// nothing marked under RequireSelected: the submit row stays out of the
	// cursor's path, so both directions wrap over it
	s := stack(RunnerOptions{Header: "pick one", SubmitLabel: "Save", RequireSelected: true}, NewPicker([]string{"claude", "codex"}))
	w := s.models[0]

	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyDown)
	assert.Equal(t, 0, s.focus)
	assert.Equal(t, 0, w.cursor, "wraps down over the gray submit row")

	key(t, s, tea.KeyUp)
	assert.Equal(t, 0, s.focus)
	assert.Equal(t, 1, w.cursor, "wraps up over the gray submit row to the last row")
}

func TestStackModel_UpdateCancelKeys(t *testing.T) {
	for _, k := range []tea.KeyType{tea.KeyEsc, tea.KeyCtrlC} {
		s := stack(RunnerOptions{Header: "pick one", SubmitLabel: "Save"}, NewPicker([]string{"claude"}))
		_, cmd := s.Update(tea.KeyMsg{Type: k})
		require.NotNil(t, cmd, k)
		cmd() // the cancel rides the cmd
		assert.Len(t, s.done, 1, "%v: the cancel rides the cmd back to Run", k)
	}
}

func TestStackModel_UpdateTracksWindowSize(t *testing.T) {
	s := stack(RunnerOptions{Header: "pick one", SubmitLabel: "Save"}, NewPicker([]string{"claude"}))
	next, cmd := s.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	updated, ok := next.(*stackModel)
	require.True(t, ok)
	assert.Equal(t, 100, updated.width)
	assert.Equal(t, 30, updated.height)
	assert.Nil(t, cmd)
}

func TestStackModel_View(t *testing.T) {
	p1 := NewPicker([]string{"claude", "codex"})
	p2 := NewPicker([]string{"pi"})
	view := stack(RunnerOptions{Header: "pick two", SubmitLabel: "save"}, p1, p2).View()

	// one chrome opens the run — the header over both pickers, a blank line
	// between them — and one submit row closes it
	assert.Contains(t, view, "pick two\n\n❯ ○ claude\n  ○ codex\n\n  ○ pi\n\n  save")
	assert.Contains(t, view, "save\n\n↑/↓ move • enter select • esc quit")
}
