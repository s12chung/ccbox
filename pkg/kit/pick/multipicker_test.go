package pick

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// multi unwraps the checklist parts of its picker
func multi(t *testing.T, p Picker) *multiPicker {
	t.Helper()
	mp, ok := p.(*multiPicker)
	require.True(t, ok)
	return mp
}

// testItems builds the checklist fixture fresh: one runtime item with a
// child, two plain — the checks ride the items, so nothing is shared
func testItems() []Item {
	return []Item{
		{Name: "go", Children: []string{"golangci-lint"}},
		{Name: "python"},
		{Name: "ruby", Hint: "from-source build"},
	}
}

func checklistChecks(t *testing.T, p Picker) []bool {
	t.Helper()
	checklist := multi(t, p).checklist
	checks := make([]bool, len(checklist))
	for i := range checklist {
		checks[i] = checklist[i].checked
	}
	return checks
}

func TestMultiPicker_UpdateEnterChecksAndReveals(t *testing.T) {
	m := NewMultiPicker(testItems(), nil)
	s := stack(RunnerOptions{Header: "pick some", SubmitLabel: "Save"}, m)

	// enter checks go and reveals its child, checked
	key(t, s, tea.KeyEnter)
	assert.Equal(t, []bool{true, false, false}, checklistChecks(t, m))
	assert.Equal(t, []bool{true}, multi(t, m).checklist[0].checks)

	// down lands on the revealed child; enter unchecks it
	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyEnter)
	assert.Equal(t, []bool{false}, multi(t, m).checklist[0].checks)

	// down lands on python; enter checks it
	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyEnter)
	assert.Equal(t, []bool{true, true, false}, checklistChecks(t, m))
}

func TestMultiPicker_UpdateEnterHidesChildren(t *testing.T) {
	m := NewMultiPicker(testItems(), []string{"go"})
	s := stack(RunnerOptions{Header: "pick some", SubmitLabel: "Save"}, m)
	assert.Len(t, multi(t, m).flatten(), 4, "go's child rides the rows while go is checked")

	// down onto the child, back up onto go, enter unchecks go — the child's row drops
	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyUp)
	key(t, s, tea.KeyEnter)
	assert.Equal(t, []bool{false, false, false}, checklistChecks(t, m))
	assert.Len(t, multi(t, m).flatten(), 3, "the hidden child leaves the rows")
}

func TestMultiPicker_UpdateEnterOnSubmitRow(t *testing.T) {
	m := NewMultiPicker(testItems(), nil)
	s := stack(RunnerOptions{Header: "pick some", SubmitLabel: "Save"}, m)
	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyDown) // onto the submit row

	_, cmd := s.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "submitting quits the program")
	var picked []string
	require.NoError(t, m.respond(&picked))
	assert.Empty(t, picked)
}

func TestMultiPicker_UpdateSubmitNeedsCheck(t *testing.T) {
	m := NewMultiPicker([]Item{{Name: "go"}, {Name: "python"}}, nil)
	s := stack(RunnerOptions{Header: "pick some", SubmitLabel: "Save", RequireSelected: true}, m)
	w := s.models[0]
	assert.False(t, s.isSubmitFocusable())

	// nothing checked: the submit row is out of the cursor's path
	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyDown)
	assert.Equal(t, 0, w.cursor)

	key(t, s, tea.KeyEnter) // checking an item arms the submit row
	assert.True(t, s.isSubmitFocusable())

	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyDown)
	assert.Equal(t, -1, s.focus, "the submit row takes the cursor once an option is checked")
}

func TestMultiPicker_Respond(t *testing.T) {
	m := NewMultiPicker(testItems(), nil)
	var picked []string
	require.NoError(t, m.respond(&picked))
	assert.NotNil(t, picked, "an empty check set stays distinct from the nil non-terminal read")
	assert.Empty(t, picked)

	// a checked item picks its name, then its checked children in row order
	m = NewMultiPicker(testItems(), []string{"go", "ruby"})
	multi(t, m).checklist[0].checks[0] = false
	picked = nil
	require.NoError(t, m.respond(&picked))
	assert.Equal(t, []string{"go", "ruby"}, picked)

	err := m.respond(new(string))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "want *[]string")
}

func TestNewMultiPicker_Checked(t *testing.T) {
	m := NewMultiPicker(testItems(), []string{"ruby", "bogus"})
	assert.Equal(t, []bool{false, false, true}, checklistChecks(t, m), "the named items pre-check, unknown names drop")
}

func TestNewMultiPicker_NoItems(t *testing.T) {
	assert.Panics(t, func() { NewMultiPicker(nil, nil) })
	assert.Panics(t, func() { NewMultiPicker([]Item{}, nil) })
}

func TestMultiPicker_View(t *testing.T) {
	header, subtext := "Select the runtimes to install", "(set in ~/.ccbox/mise.toml)"
	submitLabel := "Save to ~/.ccbox/mise.toml"
	opts := RunnerOptions{Header: header, Subtext: []string{subtext}, SubmitLabel: submitLabel}
	m := NewMultiPicker(testItems(), []string{"go"})
	s := stack(opts, m)
	s.models[0].cursor = 2 // python, past go's revealed child

	view := s.View()
	want := "Select the runtimes to install\n(set in ~/.ccbox/mise.toml)\n\n" +
		"  [x] go\n    [x] golangci-lint\n❯ [ ] python\n  [ ] ruby (from-source build)\n\n" +
		"  Save to ~/.ccbox/mise.toml\n"
	assert.Contains(t, view, want)
	assert.Contains(t, view, "↑/↓ move • enter select • esc quit")
}

func TestMultiPickerHeadless(t *testing.T) {
	// go test pipes stdin/stdout: no terminal, no prompt, no error, the nil read
	var picked []string
	runner := NewRunner(RunnerOptions{Header: "pick some", SubmitLabel: "Save"},
		NewMultiPicker([]Item{{Name: "go"}}, nil))
	require.NoError(t, runner.Run(&picked))
	assert.Nil(t, picked)
}
