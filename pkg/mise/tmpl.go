package mise

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/s12chung/ccbox/pkg/kit/tomlutil"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
)

//go:embed config.tmpl
var configTmplBody []byte

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

// SeedConfig safe-seeds the user-level mise config: the tools rendered into
// the config's marked region
func SeedConfig(path string, tools []Tool) error {
	return seed.SafeFile(path, must.Get(renderConfig(tools)))
}

// The mise config's marked generated region — the only span ccbox rewrites.
const (
	toolsBegin = "# ccbox:tools-begin"
	toolsEnd   = "# ccbox:tools-end"
)

// renderTools renders the marked region's content — the tmpl and the sync
// share it
func renderTools(tools []Tool) string {
	rows := make([]string, 0, len(tools))
	for _, t := range tools {
		rows = append(rows, t.row())
	}
	return strings.TrimSuffix(strings.Join(rows, ""), "\n")
}

var (
	configTmpl = template.Must(template.New("config.tmpl").Funcs(sections).Parse(string(configTmplBody)))
	sections   = template.FuncMap{"miseTools": renderTools}
)

// renderConfig renders the mise config for tools — SeedConfig's content. It
// executes the tmpl: each action renders its [tools] section.
func renderConfig(tools []Tool) ([]byte, error) {
	var b bytes.Buffer
	if err := configTmpl.Execute(&b, tools); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
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
