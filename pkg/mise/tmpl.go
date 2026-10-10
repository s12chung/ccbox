package mise

import (
	"bytes"
	_ "embed"
	"fmt"
	"slices"
	"strings"
	"text/template"
)

//go:embed config.tmpl
var configTmplBody []byte

// Tool is one entry of the mise config's [tools] table
type Tool struct {
	Tool    string
	Version string

	// PostInstall runs after the install (python's setuptools/wheel shim)
	PostInstall string
}

// optionalTools is the test + lint tooling every image carries, rendered at the
// bottom of the runtimes' tools
var optionalTools = []Tool{
	{Tool: "hadolint", Version: "2.14.0"},
	{Tool: "bats", Version: "1.13.0"},
}

// sections maps the tmpl's action name to its [tools] section generator; the
// tmpl's own newlines space the sections — the section's trailing one drops.
var sections = template.FuncMap{
	"miseTools": func(tools []Tool) string {
		return strings.TrimSuffix(miseToolsSection(tools), "\n")
	},
}

var configTmpl = template.Must(template.New("config.tmpl").Funcs(sections).Parse(string(configTmplBody)))

// renderConfig renders the mise config for tools — SeedConfig's content and
// BuildFS's no-host-config fallback's. It executes the tmpl: each action
// renders its [tools] section.
func renderConfig(tools []Tool) ([]byte, error) {
	var b bytes.Buffer
	if err := configTmpl.Execute(&b, tools); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// miseToolsSection renders tools with the image's optional tools at the bottom
func miseToolsSection(tools []Tool) string {
	tools = append(slices.Clone(tools), optionalTools...)

	rows := make([]string, 0, len(tools))
	for _, t := range tools {
		rows = append(rows, t.row())
	}
	return strings.Join(rows, "")
}

// row renders one [tools] entry: a plain version binding, or the inline table
// when the tool carries a postinstall
func (t Tool) row() string {
	if t.PostInstall == "" {
		return fmt.Sprintf("%s = %q\n", t.Tool, t.Version)
	}
	return fmt.Sprintf("%s = { version = %q, postinstall = %q }\n", t.Tool, t.Version, t.PostInstall)
}
