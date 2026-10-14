package mise

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
)

// SeedConfig safe-seeds the user-level mise config: the tools rendered into
// the config's marked region
func SeedConfig(path string, tools []Tool) error {
	var b bytes.Buffer
	if err := configTmpl.Execute(&b, tools); err != nil {
		return err
	}
	return seed.SafeFile(path, b.Bytes())
}

var (
	//go:embed config.tmpl
	configTmplBody []byte
	configTmpl     = template.Must(template.New("config.tmpl").Funcs(sections).Parse(string(configTmplBody)))
	sections       = template.FuncMap{"miseTools": renderTools}
)

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

// The mise config's marked generated region — the only span ccbox rewrites.
const (
	toolsBegin = "# ccbox:tools-begin"
	toolsEnd   = "# ccbox:tools-end"
)

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
