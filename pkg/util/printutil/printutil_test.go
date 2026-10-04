package printutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSection_Lines(t *testing.T) {
	assert.Equal(t, []string{"# alias:"}, Section{Header: "alias:"}.lines())
	assert.Equal(t,
		[]string{"# alias:", "#   - a.dev", "#   - b.dev"},
		Section{Header: "alias:", Items: []string{"a.dev", "b.dev"}}.lines(),
	)
	assert.Equal(t,
		[]string{"#   field:", "#     - a"},
		Section{Header: "field:", Items: []string{"a"}, Depth: 1}.lines(),
	)
}

func TestRender(t *testing.T) {
	assert.Nil(t, Render(nil))

	// top-level sections blank-line apart; nested ones hug the section above them
	assert.Equal(t, []string{
		"# parent:",
		"#   child:",
		"#     - a",
		"",
		"# other:",
		"#   - b",
	}, Render([]Section{
		{Header: "parent:"},
		{Header: "child:", Items: []string{"a"}, Depth: 1},
		{Header: "other:", Items: []string{"b"}},
	}))
}
