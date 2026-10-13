package pick

import (
	"slices"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMultiModel builds the checklist model for the tests; NewMultiSelect inlines
// the literal
func newMultiModel(header string, subtext []string, submitLabel string, requireSelected bool, options, checked []string) *multiModel {
	m := &multiModel{picker: newPicker(header, subtext, submitLabel, requireSelected, options), checked: make([]bool, len(options))}
	for _, opt := range checked {
		if i := slices.Index(m.options, opt); i >= 0 {
			m.checked[i] = true
		}
	}
	return m
}

func TestMultiModel_UpdateEnterToggles(t *testing.T) {
	m := newMultiModel("pick some", nil, "Save", false, []string{"go", "python", "ruby"}, nil)

	m = key(t, m, tea.KeyEnter)
	assert.Equal(t, []bool{true, false, false}, m.checked)

	m = key(t, key(t, m, tea.KeyDown), tea.KeyEnter)
	assert.Equal(t, []bool{true, true, false}, m.checked)

	m = key(t, key(t, m, tea.KeyUp), tea.KeyEnter)
	assert.Equal(t, []bool{false, true, false}, m.checked, "enter unchecks too")
}

func TestMultiModel_UpdateEnterOnSubmitRow(t *testing.T) {
	m := newMultiModel("pick some", nil, "Save", false, []string{"go", "python"}, nil)
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
	m := newMultiModel("pick some", nil, "Save", true, []string{"go", "python"}, nil)
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
	m := newMultiModel("pick some", nil, "Save", false, []string{"go", "python", "ruby"}, nil)
	picked, err := m.Response()
	require.NoError(t, err)
	assert.NotNil(t, picked, "an empty check set stays distinct from the nil non-terminal read")
	assert.Empty(t, picked)

	m.checked[2], m.checked[0] = true, true
	picked, err = m.Response()
	require.NoError(t, err)
	assert.Equal(t, []string{"go", "ruby"}, picked, "picked follows the options' order")
}

func TestNewMultiSelect_Checked(t *testing.T) {
	m := newMultiModel("pick some", nil, "Save", false, []string{"go", "python", "ruby"}, []string{"ruby", "bogus"})
	assert.Equal(t, []bool{false, false, true}, m.checked, "the named options pre-check, unknown names drop")
}

func TestMultiModel_View(t *testing.T) {
	header, subtext := "Select the runtimes to install", "(set in ~/.ccbox/mise.toml)"
	submitLabel := "Save to ~/.ccbox/mise.toml"
	m := newMultiModel(header, []string{subtext}, submitLabel, false, []string{"go", "python", "ruby"}, nil)
	m.checked[0] = true
	m.cursor = 2

	view := m.View()
	want := "Select the runtimes to install\n(set in ~/.ccbox/mise.toml)\n\n" +
		"  [x] go\n  [ ] python\n❯ [ ] ruby\n\n" +
		"  Save to ~/.ccbox/mise.toml\n"
	assert.Contains(t, view, want)
	assert.Contains(t, view, "↑/↓ move • enter check • esc quit")
}

func TestNewMultiSelect_NoOptions(t *testing.T) {
	assert.Panics(t, func() { NewMultiSelect("pick some", nil, "Save", false, nil, nil) })
	assert.Panics(t, func() { NewMultiSelect("pick some", nil, "Save", false, []string{}, nil) })
}

func TestMultiSelectHeadless(t *testing.T) {
	// go test pipes stdin/stdout: no terminal, no prompt, no error
	picked, err := RunWithResponse(NewMultiSelect("pick some", nil, "Save", false, []string{"go"}, nil))
	require.NoError(t, err)
	assert.Nil(t, picked)
}
