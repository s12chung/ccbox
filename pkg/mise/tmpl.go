package mise

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/pelletier/go-toml/v2"

	"github.com/s12chung/ccbox/pkg/kit/tomlutil"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
)

// ConfigSet is the mise config's generated content, carried in its marked
// regions — its tools from ResolvedTools
type ConfigSet struct {
	Tools    []ResolvedTool
	Packages []string // the runtimes' apt libs; rows render apt-prefixed
}

// renderTools renders the marked region's content — the tmpl and the sync
// share it
func (c ConfigSet) renderTools() string {
	rows := make([]string, 0, len(c.Tools))
	for _, t := range c.Tools {
		rows = append(rows, t.row())
	}
	return strings.TrimSuffix(strings.Join(rows, ""), "\n")
}

// renderPackages renders the marked region's content — the tmpl and the sync
// share it
func (c ConfigSet) renderPackages() string {
	rows := make([]string, 0, len(c.Packages))
	for _, pkg := range c.Packages {
		rows = append(rows, fmt.Sprintf("%s = %q\n", tomlutil.Key("apt:"+pkg), "latest"))
	}
	return strings.TrimSuffix(strings.Join(rows, ""), "\n")
}

// SeedConfig safe-seeds the user-level mise config: the config set rendered into
// the config's marked regions
func SeedConfig(path string, configSet ConfigSet) error {
	var b bytes.Buffer
	if err := configTmpl.Execute(&b, configSet); err != nil {
		return err
	}
	return seed.SafeFile(path, b.Bytes())
}

var (
	//go:embed config.tmpl
	configTmplBody []byte
	configTmpl     = template.Must(template.New("config.tmpl").Funcs(sections).Parse(string(configTmplBody)))
	sections       = template.FuncMap{"miseTools": ConfigSet.renderTools, "misePackages": ConfigSet.renderPackages}
)

// Sync syncs the generated regions of the config the build picks — project
// over user; with neither, there's nothing to sync and it errors.
func Sync(projectDir string, configSet ConfigSet) error {
	path := PickedConfigPath(projectDir)
	if path == "" {
		return fmt.Errorf("mise: no mise config to sync — expected a project config at %s or the user config at %s",
			ProjectConfigPath(projectDir), UserConfigPath())
	}
	return syncConfig(path, configSet)
}

// syncConfig rewrites the picked config's marked regions to configSet's content.
func syncConfig(path string, configSet ConfigSet) error {
	body, err := os.ReadFile(path) // #nosec G304 -- the config's own path
	if err != nil {
		return err
	}
	synced, err := setRegions(string(body), configSet)
	if err != nil {
		return fmt.Errorf("mise: %s: %w", path, err)
	}
	var parsed map[string]any
	if err := toml.Unmarshal([]byte(synced), &parsed); err != nil {
		return fmt.Errorf("mise: %s: %w — delete the file to reseed it", path, err)
	}
	return osutil.AtomicWriteFile(path, []byte(synced))
}

// The mise config's marked generated regions — the only spans ccbox rewrites.
const (
	toolsBegin = "# ccbox:tools-begin"
	toolsEnd   = "# ccbox:tools-end"

	packagesBegin = "# ccbox:packages-begin"
	packagesEnd   = "# ccbox:packages-end"
)

// setRegions replaces body's marked regions with their render, everything
// outside byte-for-byte
func setRegions(body string, configSet ConfigSet) (string, error) {
	regions := []markedRegion{
		{toolsBegin, toolsEnd, configSet.renderTools()},
		{packagesBegin, packagesEnd, configSet.renderPackages()},
	}
	for _, r := range regions {
		var err error
		if body, err = setRegion(body, r); err != nil {
			return "", err
		}
	}
	return body, nil
}

// markedRegion is one marked span with its region's rendered content.
type markedRegion struct {
	begin  string
	end    string
	region string
}

// setRegion replaces body's marked region with region, everything outside
// byte-for-byte.
func setRegion(body string, r markedRegion) (string, error) {
	beginLine := strings.Index(body, r.begin+"\n")
	endLine := strings.Index(body, "\n"+r.end)
	if beginLine < 0 || endLine < 0 || endLine < beginLine {
		return "", fmt.Errorf("no %s/%s markers — delete the file to reseed it", r.begin, r.end)
	}
	return body[:beginLine+len(r.begin)+1] + r.region + body[endLine:], nil
}
