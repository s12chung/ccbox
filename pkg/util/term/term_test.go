package term

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestConfirm(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "y", input: "y\n", want: true},
		{name: "yes padded and capped", input: "  YES \n", want: true},
		{name: "n", input: "n\n"},
		{name: "empty line", input: "\n"},
		{name: "garbage", input: "maybe\n"},
		{name: "eof reads as no", input: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testutil.Stdin(t, tc.input)
			assert.Equal(t, tc.want, Confirm("proceed?"))
		})
	}
}
