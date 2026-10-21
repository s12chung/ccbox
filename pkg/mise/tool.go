package mise

import (
	"fmt"
	"strings"

	"github.com/s12chung/ccbox/pkg/kit/tomlutil"
)

// Tool is one entry of the mise config's [tools] table — the version binding
// resolves at generation: the config pins the minor, the lock flexes the patch
type Tool struct {
	Name        string
	PostInstall string // runs after the install

	Comment string // renders above the row when the tool needs a why
	Hint    string // the mise tools picker's gray note after the name
}

// ResolvedTool is a Tool fully resolved to its config row's final values —
// the render reads, never calculates
type ResolvedTool struct {
	Name        string
	Pin         string
	PostInstall string // the final postinstall cmd; "" when the tool has none
	Comment     string // renders above the row when the tool needs a why
}

// ResolvedTools resolves the tools to their config rows — each written pin
// riding, the rest fetched
func ResolvedTools(tools []Tool, state ToolState) ([]ResolvedTool, error) {
	resolved := make([]ResolvedTool, 0, len(tools))
	for _, t := range tools {
		rt, err := newResolvedTool(t, state)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, rt)
	}
	return resolved, nil
}

// newResolvedTool fully resolves t: the tool's pin, then its postinstall
func newResolvedTool(t Tool, state ToolState) (ResolvedTool, error) {
	pin, err := state.PinFor(t.Name)
	if err != nil {
		return ResolvedTool{}, err
	}
	return ResolvedTool{Name: t.Name, Pin: pin, PostInstall: t.PostInstall, Comment: t.Comment}, nil
}

// row renders one [tools] entry: the comment line when the tool carries one,
// then the version binding, or the inline table when it carries a postinstall
func (t ResolvedTool) row() string {
	var b strings.Builder
	if t.Comment != "" {
		fmt.Fprintln(&b, t.Comment)
	}
	if t.PostInstall == "" {
		fmt.Fprintf(&b, "%s = %q\n", tomlutil.Key(t.Name), t.Pin)
	} else {
		fmt.Fprintf(&b, "%s = { version = %q, postinstall = %q }\n",
			tomlutil.Key(t.Name), t.Pin, t.PostInstall)
	}
	return b.String()
}
