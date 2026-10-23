package pick

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// selectAllStack is the select-all row's PickerStack over its pickers
func selectAllStack(pickers ...SelectAllPicker) []Picker {
	return SelectAll{Pickers: pickers}.PickerStack()
}

func TestSelectAll_NoPickers(t *testing.T) {
	// the zero SelectAll is the row: nothing to read, nothing to drive
	sa := SelectAll{}.PickerStack()[0]
	assert.Contains(t, sa.render(0, false)[0], "[ ] Select All", "no pickers, nothing read checked")
	require.NoError(t, sa.respond(nil))
}

func TestSelectAll_View(t *testing.T) {
	m := NewMultiPicker(testItems(), nil)
	s := stack(RunnerOptions{Header: "pick some", SubmitLabel: "Save"}, selectAllStack(m)...)

	// the one row opens the run, its blank line keeping it apart from the
	// checklist under it — nothing checked, the row unchecked
	assert.Equal(t, []string{
		"  pick some",
		"",
		"  ❯ [ ] Select All",
		"",
		"    [ ] go",
		"    [ ] python",
		"    [ ] ruby (from-source build)",
		"",
		"    Save",
		"",
		"  ↑/↓ move • enter select • esc quit",
	}, trimmedLines(s.View()))
}

func TestSelectAll_Render(t *testing.T) {
	// the check reads off the pickers at render: all of them checked checks
	// the row — the hidden child counting with its item
	sa := selectAllStack(NewMultiPicker(testItems(), []string{"go", "golangci-lint", "python", "ruby"}))[0]
	assert.Contains(t, sa.render(0, false)[0], "[x] Select All")

	sa = selectAllStack(NewMultiPicker(testItems(), []string{"go", "golangci-lint", "python"}))[0]
	assert.Contains(t, sa.render(0, false)[0], "[ ] Select All", "ruby unchecked leaves the row unchecked")
}

func TestSelectAll_EnterChecksEveryRow(t *testing.T) {
	m := NewMultiPicker(testItems(), nil)
	s := stack(RunnerOptions{Header: "pick some", SubmitLabel: "Save"}, selectAllStack(m)...)

	key(t, s, tea.KeyEnter) // enter on the select-all row
	assert.Equal(t, []bool{true, true, true}, checklistChecks(t, m))
	assert.Equal(t, []bool{true}, multi(t, m).checklist[0].checks, "the hidden child checks with its item")
	assert.Contains(t, s.models[0].render(0, false)[0], "[x] Select All", "all checked checks the row with them")

	// down off the row crosses into the checklist; unchecking go unchecks the row
	key(t, s, tea.KeyDown)
	key(t, s, tea.KeyEnter)
	assert.Equal(t, []bool{false, true, true}, checklistChecks(t, m))
	assert.Contains(t, s.models[0].render(0, false)[0], "[ ] Select All", "the row's check is the pickers', never its own")
}

func TestSelectAll_Picked(t *testing.T) {
	sa := selectAllStack(NewMultiPicker(testItems(), nil))[0]
	assert.True(t, sa.picked(), "the row never holds the submit back")
}

func TestSelectAll_Respond(t *testing.T) {
	sa := selectAllStack(NewMultiPicker(testItems(), nil))[0]
	require.NoError(t, sa.respond(nil))

	err := sa.respond(new(bool))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reads nothing")
}
