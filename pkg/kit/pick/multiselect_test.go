package pick

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMultiModel builds the checklist model for the tests; NewMultiSelect inlines
// the literal
func newMultiModel(header string, subtext []string, submitLabel string, requireSelected bool, options []string) *multiModel {
	return &multiModel{picker: newPicker(header, subtext, submitLabel, requireSelected, options), checked: make([]bool, len(options))}
}

func TestMultiModel_UpdateEnterToggles(t *testing.T) {
	m := newMultiModel("pick some", nil, "Save", false, []string{"go", "python", "ruby"})

	m = key(t, m, tea.KeyEnter)
	assert.Equal(t, []bool{true, false, false}, m.checked)

	m = key(t, key(t, m, tea.KeyDown), tea.KeyEnter)
	assert.Equal(t, []bool{true, true, false}, m.checked)

	m = key(t, key(t, m, tea.KeyUp), tea.KeyEnter)
	assert.Equal(t, []bool{false, true, false}, m.checked, "enter unchecks too")
}

func TestMultiModel_UpdateEnterOnSubmitRow(t *testing.T) {
	m := newMultiModel("pick some", nil, "Save", false, []string{"go", "python"})
	m.cursor = len(m.options) // the submit row

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated, ok := next.(*multiModel)
	require.True(t, ok)
	require.NotNil(t, cmd, "submitting quits the program")
	picked, err := updated.Response()
	require.NoError(t, err)
	assert.Empty(t, picked)
}

func TestMultiModel_UpdateSubmitNeedsCheck(t *testing.T) {
	m := newMultiModel("pick some", nil, "Save", true, []string{"go", "python"})
	assert.False(t, m.submitOn())

	// nothing checked: the gray submitLabel row is out of the cursor's path
	m = key(t, key(t, m, tea.KeyDown), tea.KeyDown)
	assert.Equal(t, 0, m.cursor)

	m = key(t, m, tea.KeyEnter) // checking an option arms the submit row
	assert.True(t, m.submitOn())

	m = key(t, key(t, m, tea.KeyDown), tea.KeyDown)
	assert.Equal(t, 2, m.cursor, "the submit row takes the cursor once an option is checked")
}

func TestMultiModel_Picked(t *testing.T) {
	m := newMultiModel("pick some", nil, "Save", false, []string{"go", "python", "ruby"})
	picked, err := m.Response()
	require.NoError(t, err)
	assert.NotNil(t, picked, "an empty check set stays distinct from the nil non-terminal read")
	assert.Empty(t, picked)

	m.checked[2], m.checked[0] = true, true
	picked, err = m.Response()
	require.NoError(t, err)
	assert.Equal(t, []string{"go", "ruby"}, picked, "picked follows the options' order")
}

func TestMultiModel_View(t *testing.T) {
	header, subtext := "Select the runtimes to install", "(set in ~/.ccbox/config/prov.yaml)"
	submitLabel := "Save to ~/.ccbox/config/prov.yaml"
	m := newMultiModel(header, []string{subtext}, submitLabel, false, []string{"go", "python", "ruby"})
	m.checked[0] = true
	m.cursor = 2

	view := m.View()
	assert.Contains(t, view,
		"Select the runtimes to install\n(set in ~/.ccbox/config/prov.yaml)\n\n  [x] go\n  [ ] python\n❯ [ ] ruby\n\n  Save to ~/.ccbox/config/prov.yaml\n")
	assert.Contains(t, view, "↑/↓ move • enter check • esc quit")
}

func TestNewMultiSelect_NoOptions(t *testing.T) {
	assert.Panics(t, func() { NewMultiSelect("pick some", nil, "Save", false, nil) })
	assert.Panics(t, func() { NewMultiSelect("pick some", nil, "Save", false, []string{}) })
}

func TestMultiSelectHeadless(t *testing.T) {
	// go test pipes stdin/stdout: no terminal, no prompt, no error
	picked, err := RunWithResponse(NewMultiSelect("pick some", nil, "Save", false, []string{"go"}))
	require.NoError(t, err)
	assert.Nil(t, picked)
}
