package mise

import (
	"fmt"
	"strings"

	"github.com/s12chung/ccbox/pkg/kit/tomlutil"
)

// Tool is one entry of the mise config's [tools] table
type Tool struct {
	Tool        string
	Version     string
	PostInstall string // runs after the install

	Comment string // renders above the row when the tool needs a why
	Hint    string // the mise tools picker's gray note after the name
}

// row renders one [tools] entry: the comment line when the tool carries one,
// then a plain version binding, or the inline table when it carries a postinstall
func (t Tool) row() string {
	var b strings.Builder
	if t.Comment != "" {
		fmt.Fprintln(&b, t.Comment)
	}
	if t.PostInstall == "" {
		fmt.Fprintf(&b, "%s = %q\n", tomlutil.Key(t.Tool), t.Version)
	} else {
		fmt.Fprintf(&b, "%s = { version = %q, postinstall = %q }\n", tomlutil.Key(t.Tool), t.Version, t.PostInstall)
	}
	return b.String()
}

// renderTools renders the marked region's content — the tmpl and the sync
// share it
func renderTools(tools []Tool) string {
	rows := make([]string, 0, len(tools))
	for _, t := range tools {
		rows = append(rows, t.row())
	}
	return strings.TrimSuffix(strings.Join(rows, ""), "\n")
}
