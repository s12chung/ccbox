package mise

import (
	_ "embed"
	"fmt"
	"maps"
	"slices"
	"strings"
	"text/template"

	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/util/must"
)

//go:embed config.tmpl
var configTmpl []byte

// miseToolOrder is the language-runtimes section's display order — the config's
// long-standing layout; mise resolves the table's entries order-free.
var miseToolOrder = []string{"node", "python", "ruby", "go"}

// sharedLintMiseTools is the test + lint tooling every image carries, before the
// runtimes' own ecosystem tooling
var sharedLintMiseTools = []runtime.MiseTool{
	{Tool: "shellcheck", Version: "0.11.0"},
	{Tool: "hadolint", Version: "2.14.0"},
	{Tool: "bats", Version: "1.13.0"},
}

// RenderConfig renders the mise config from the runtimes' tools plus the shared
// tooling every image carries — SeedConfig's content and BuildFS's no-host-config
// fallback's.
func RenderConfig() []byte { return must.Get(renderConfig(configTmpl)) }

// sections generates each template action's content; the tmpl must render every
// one (renderConfig hard-errors otherwise).
var sections = map[string]func() string{
	"runtimes":  runtimesSection,
	"lintTools": lintToolsSection,
}

// renderConfig executes the tmpl body: each action renders its [tools] section,
// and a tmpl that never renders a section is a hard error — the tmpl and this
// composer must agree on every section.
func renderConfig(body []byte) ([]byte, error) {
	seen := make(map[string]bool, len(sections))
	funcs := make(template.FuncMap, len(sections))
	for name, generate := range sections {
		funcs[name] = func() string {
			seen[name] = true
			// the tmpl's own newlines space the sections; the sections' trailing one drops
			return strings.TrimSuffix(generate(), "\n")
		}
	}
	tmpl, err := template.New("config.tmpl").Funcs(funcs).Parse(string(body))
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, nil); err != nil {
		return nil, err
	}
	var missing []string
	for _, name := range slices.Sorted(maps.Keys(sections)) {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("mise: the tmpl never renders: %s", strings.Join(missing, ", "))
	}
	return []byte(b.String()), nil
}

// runtimesSection renders the language-runtimes section: each runtime's own tool,
// in the config's long-standing display order
func runtimesSection() string {
	byTool := make(map[string]runtime.MiseTool)
	for _, r := range runtime.All() {
		byTool[r.MiseTool.Tool] = r.MiseTool
	}
	tools := make([]runtime.MiseTool, 0, len(miseToolOrder))
	for _, name := range miseToolOrder {
		tools = append(tools, byTool[name])
	}
	return renderSection("# Language runtimes", tools)
}

// lintToolsSection renders the test + lint section: the shared tooling, then the
// runtimes' ecosystem tooling
func lintToolsSection() string {
	return renderSection("# Test + lint tooling", slices.Concat(sharedLintMiseTools, optionalMiseTools(runtime.All())))
}

// renderSection renders one comment-titled run of [tools] entries, key-aligned
// to the section's longest key
func renderSection(comment string, tools []runtime.MiseTool) string {
	pad := 0
	for _, t := range tools {
		pad = max(pad, len(t.Tool))
	}
	var b strings.Builder
	b.WriteString(comment + "\n")
	for _, t := range tools {
		b.WriteString(miseToolRow(pad, t))
	}
	return b.String()
}

// miseToolRow renders one [tools] entry: a plain version binding, or python-style
// inline table when the tool carries a postinstall (its why-comment rides above).
func miseToolRow(pad int, t runtime.MiseTool) string {
	row := fmt.Sprintf("%-*s = %q\n", pad, t.Tool, t.Version)
	if t.PostInstall == "" {
		return row
	}
	const postinstallComment = "# postinstall re-adds setuptools/wheel CPython 3.12+ dropped (setuptools = 3.13's distutils shim)\n"
	return postinstallComment + fmt.Sprintf("%-*s = { version = %q, postinstall = %q }\n", pad, t.Tool, t.Version, t.PostInstall)
}

// optionalMiseTools collects the runtimes' ecosystem tooling, runtime-sorted
func optionalMiseTools(runtimes []runtime.Runtime) []runtime.MiseTool {
	var tools []runtime.MiseTool
	for _, r := range runtimes {
		tools = append(tools, r.MiseToolsOptional...)
	}
	return tools
}
