package mise

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/s12chung/ccbox/pkg/kit/tomlutil"
	"github.com/s12chung/ccbox/pkg/util/must"
)

// Tool is one entry of the mise config's [tools] table — the image's recipe
type Tool struct {
	Name string

	// PostInstallCmdTmpl is the postinstall as a text/template over PostInstallTools
	PostInstallCmdTmpl string
	// PostInstallTools are the postinstall's packages — nil declares none, an
	// empty slice answers every package dropped
	PostInstallTools []Tool

	Comment string // renders above the row when the tool needs a why
	Hint    string // the mise tools picker's gray note after the name
}

// postInstallCmd is the tool's final postinstall cmd: the config's written one
// as-is, else the tmpl over the tool's packages
func (t Tool) postInstallCmd(state ToolState) (string, error) {
	if post := state.PostInstalls[t.Name]; post != "" {
		return post, nil // the written post carries its packages' pins — they never fetch, never render
	}
	if t.PostInstallTools != nil && len(t.PostInstallTools) == 0 {
		return "", nil // every package dropped — the plain pin
	}
	pkgs := make([]postInstallPkg, 0, len(t.PostInstallTools))
	for _, pkg := range t.PostInstallTools {
		version, err := state.PinFor(pkg.Name)
		if err != nil {
			return "", err
		}
		pkgs = append(pkgs, postInstallPkg{Tool: pkg.Name, Version: version})
	}
	tmpl := template.Must(template.New(t.Name).Parse(t.PostInstallCmdTmpl))
	var b strings.Builder
	must.Do(tmpl.Execute(&b, pkgs)) // the tmpl is authored and its data resolved — neither fails here
	return b.String(), nil
}

// ResolvedTool is a Tool fully resolved to its config row's final values —
// the render reads, never calculates
type ResolvedTool struct {
	Name        string
	Pin         string
	PostInstall string // the final postinstall cmd; "" when the tool has none
	Comment     string // renders above the row when the tool needs a why
}

// ResolvedTools resolves the tools to their config rows
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

// newResolvedTool fully resolves t: the pin, then the postinstall cmd
func newResolvedTool(t Tool, state ToolState) (ResolvedTool, error) {
	rt := ResolvedTool{Name: t.Name, Comment: t.Comment}
	var err error
	rt.Pin, err = state.PinFor(t.Name)
	if err != nil {
		return ResolvedTool{}, err
	}
	if rt.PostInstall, err = t.postInstallCmd(state); err != nil {
		return ResolvedTool{}, err
	}
	return rt, nil
}

// row renders one [tools] entry
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

// postInstallPkg is one resolved postinstall package
type postInstallPkg struct {
	Tool    string
	Version string
}

// Name is the tool's name sans its backend prefix — pypi:setuptools's setuptools
func (p postInstallPkg) Name() string {
	return p.Tool[strings.LastIndex(p.Tool, ":")+1:]
}
