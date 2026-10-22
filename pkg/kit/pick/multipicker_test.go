package pick

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// multi unwraps the checklist parts of its picker
func multi(t *testing.T, p Picker) *multiPicker[testModel] {
	t.Helper()
	mp, ok := p.(*multiPicker[testModel])
	require.True(t, ok)
	return mp
}

// testItems builds the checklist fixture fresh: one runtime item with a
// child, two plain — the checks live on the items, so nothing is shared
func testItems() []Item[testModel] {
	return []Item[testModel]{
		{Model: testModel{name: "go"}, Children: []testModel{{name: "golangci-lint"}}},
		{Model: testModel{name: "python"}},
		{Model: testModel{name: "ruby", hint: "from-source build"}},
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

func TestFlatten(t *testing.T) {
	items := []Item[testModel]{
		{Model: testModel{name: "go"}, Children: []testModel{{name: "golangci-lint"}}},
		{Model: testModel{name: "ruby"}},
	}
	assert.Equal(t, []testModel{{name: "go"}, {name: "golangci-lint"}, {name: "ruby"}}, Flatten(items))
	assert.Empty(t, Flatten([]Item[testModel](nil)))
}

func TestFlattenFunc(t *testing.T) {
	items := []Item[testModel]{
		{Model: testModel{name: "go"}, Children: []testModel{{name: "golangci-lint"}}},
		{Model: testModel{name: "ruby"}},
	}
	got := FlattenFunc(items, func(model testModel, children []testModel) []string {
		if len(children) == 0 {
			return []string{model.name}
		}
		return []string{model.name + " + " + children[0].name}
	})
	assert.Equal(t, []string{"go + golangci-lint", "ruby"}, got)
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
	assert.Len(t, multi(t, m).flatten(), 4, "go's child joins the rows while go is checked")

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
	var picked []Item[testModel]
	require.NoError(t, m.respond(&picked))
	assert.Empty(t, picked)
}

func TestMultiPicker_UpdateSubmitNeedsCheck(t *testing.T) {
	m := NewMultiPicker([]Item[testModel]{{Model: testModel{name: "go"}}, {Model: testModel{name: "python"}}}, nil)
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
	var picked []Item[testModel]
	require.NoError(t, m.respond(&picked))
	assert.NotNil(t, picked, "an empty check set stays distinct from the nil non-terminal read")
	assert.Empty(t, picked)

	// a checked item picks its model with its checked children alone —
	// an unchecked child drops, an unchecked item drops its children with it
	m = NewMultiPicker(testItems(), []string{"go", "ruby"})
	multi(t, m).checklist[0].checks[0] = false
	picked = nil
	require.NoError(t, m.respond(&picked))
	assert.Equal(t, []Item[testModel]{
		{Model: testModel{name: "go"}},
		{Model: testModel{name: "ruby", hint: "from-source build"}},
	}, picked)

	// go's child comes along once checked
	m = NewMultiPicker(testItems(), []string{"go"})
	picked = nil
	require.NoError(t, m.respond(&picked))
	assert.Equal(t, []Item[testModel]{
		{Model: testModel{name: "go"}, Children: []testModel{{name: "golangci-lint"}}},
	}, picked)

	err := m.respond(new(string))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "want *[]pick.Item")
}

func TestNewMultiPicker_Checked(t *testing.T) {
	t.Run("the named items pre-check, unknown names drop", func(t *testing.T) {
		m := NewMultiPicker(testItems(), []string{"ruby", "bogus"})
		assert.Equal(t, []bool{false, false, true}, checklistChecks(t, m))
	})

	t.Run("a child's name pre-checks it past its item's RevealUnchecked", func(t *testing.T) {
		m := NewMultiPicker([]Item[testModel]{
			{Model: testModel{name: "node"}, Children: []testModel{{name: "oxlint"}}, RevealUnchecked: true},
		}, []string{"oxlint"})
		assert.Equal(t, []bool{true}, multi(t, m).checklist[0].checks)
	})
}

func TestNewMultiPicker_Disabled(t *testing.T) {
	items := []Item[testModel]{
		{Model: testModel{name: "node"}, Children: []testModel{{name: "oxlint"}}, IsDisabled: true},
		{Model: testModel{name: "go"}},
	}

	t.Run("a locked item is checked from the start", func(t *testing.T) {
		m := NewMultiPicker(items, nil)
		assert.Equal(t, []bool{true, false}, checklistChecks(t, m))
	})

	t.Run("enter never unchecks the locked row", func(t *testing.T) {
		m := NewMultiPicker(items, nil)
		s := stack(RunnerOptions{Header: "pick some", SubmitLabel: "Save"}, m)
		key(t, s, tea.KeyEnter)
		assert.Equal(t, []bool{true, false}, checklistChecks(t, m))
	})

	t.Run("the locked row renders checked", func(t *testing.T) {
		m := NewMultiPicker(items, nil)
		assert.Contains(t, m.render(0, true)[0], "[x] node")
	})
}

func TestNewMultiPicker_RevealUnchecked(t *testing.T) {
	items := []Item[testModel]{
		{Model: testModel{name: "go"}, Children: []testModel{{name: "golangci-lint"}}, RevealUnchecked: true},
	}
	m := NewMultiPicker(items, []string{"go"})
	assert.Equal(t, []bool{false}, multi(t, m).checklist[0].checks, "the reveal starts unchecked")
}

func TestNewMultiPicker_NoItems(t *testing.T) {
	assert.Panics(t, func() { NewMultiPicker[testModel](nil, nil) })
	assert.Panics(t, func() { NewMultiPicker([]Item[testModel]{}, nil) })
}

func TestMultiPicker_View(t *testing.T) {
	header, subtext := "Select the runtimes to install", "(set in ~/.ccbox/mise.toml)"
	submitLabel := "Save to ~/.ccbox/mise.toml"
	opts := RunnerOptions{Header: header, Subtext: []string{subtext}, SubmitLabel: submitLabel}
	m := NewMultiPicker(testItems(), []string{"go"})
	s := stack(opts, m)
	s.models[0].cursor = 2 // python, past go's revealed child

	// every row carries the run's 2-space left padding, over the rows' own indents
	assert.Equal(t, []string{
		"  Select the runtimes to install",
		"  (set in ~/.ccbox/mise.toml)",
		"",
		"    [x] go",
		"      [x] golangci-lint",
		"  ❯ [ ] python",
		"    [ ] ruby (from-source build)",
		"",
		"    Save to ~/.ccbox/mise.toml",
		"",
		"  ↑/↓ move • enter select • esc quit",
	}, trimmedLines(s.View()))
}

func TestMultiPickerHeadless(t *testing.T) {
	// go test pipes stdin/stdout: no terminal, no prompt, no error, the nil read
	var picked []Item[testModel]
	runner := NewRunner(RunnerOptions{Header: "pick some", SubmitLabel: "Save"},
		NewMultiPicker([]Item[testModel]{{Model: testModel{name: "go"}}}, nil))
	require.NoError(t, runner.Run(&picked))
	assert.Nil(t, picked)
}
