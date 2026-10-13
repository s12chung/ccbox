package mise

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"slices"
	"strings"
	"text/template"

	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
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
	"miseTools": renderTools,
}

// renderTools renders the marked region's content — the tmpl and the sync share it
func renderTools(tools []Tool) string { return strings.TrimSuffix(miseToolsSection(tools), "\n") }

// The mise config's marked generated region — the only span ccbox rewrites.
const (
	toolsBegin = "# ccbox:tools-begin"
	toolsEnd   = "# ccbox:tools-end"
)

var configTmpl = template.Must(template.New("config.tmpl").Funcs(sections).Parse(string(configTmplBody)))

// SeedConfig safe-seeds the user-level mise config: tools over the pinned defaults
func SeedConfig(path string, tools []Tool) error {
	return seed.SafeFile(path, must.Get(renderConfig(tools)))
}

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

// SyncTools syncs the generated tools region of the config the build picks —
// project over user; with neither, there's nothing to sync and it errors.
func SyncTools(projectDir string, tools []Tool) error {
	path := PickedConfigPath(projectDir)
	if path == "" {
		return fmt.Errorf("mise: no mise config to sync — expected a project config at %s or the user config at %s",
			ProjectConfigPath(projectDir), UserConfigPath())
	}
	return syncTools(path, tools)
}

// syncTools rewrites the marked region to tools' rows.
func syncTools(path string, tools []Tool) error {
	body, err := os.ReadFile(path) // #nosec G304 -- the config's own path
	if err != nil {
		return err
	}
	synced, err := setToolsRegion(string(body), renderTools(tools))
	if err != nil {
		return fmt.Errorf("mise: %s: %w", path, err)
	}
	return osutil.AtomicWriteFile(path, []byte(synced))
}

// setToolsRegion replaces body's marked region with toolsRegion, everything
// outside byte-for-byte.
func setToolsRegion(body, toolsRegion string) (string, error) {
	beginLine := strings.Index(body, toolsBegin+"\n")
	endLine := strings.Index(body, "\n"+toolsEnd)
	if beginLine < 0 || endLine < 0 || endLine < beginLine {
		return "", fmt.Errorf("no %s/%s markers — delete the file to reseed it", toolsBegin, toolsEnd)
	}
	return body[:beginLine+len(toolsBegin)+1] + toolsRegion + body[endLine:], nil
}
