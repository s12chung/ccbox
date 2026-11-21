package share

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"text/template"

	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/scratch"
	"github.com/s12chung/ccbox/pkg/util/seed"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

//go:embed default.tmpl.md
var defaultTmplMd []byte

// SafeSeedAgentsMd seeds the shared template when missing; existing files are
// never touched
func SafeSeedAgentsMd() error { return seed.SafeFile(userAgentsTmplMdPath(), defaultTmplMd) }

// AgentsMd returns the Binder for the CLI's AGENTS doc; isProxied answers the
// template's IsProxied.
func AgentsMd(cliName string, isProxied bool) scratch.Binder {
	mount := AgentsMdScratchMount(cliName)
	return scratch.Binder{
		Share: scratch.Share{
			Name:     "agents",
			RealPath: realCliFile(cliName),
			Content:  scratch.File{Source: func() ([]byte, error) { return agentsMdSource(cliName, isProxied) }},
			Sync:     scratch.Symlink{Mount: mount},
		},
		BindPath: mount,
	}
}

// AgentsMdScratchMount is the CLI's scratch copy's container path:
// <ContainerHome>/.ccbox/tmp/agents/<cli_name>/AGENTS.md
func AgentsMdScratchMount(cliName string) string {
	return path.Join(pkginfo.ContainerHome, ".ccbox", "tmp", "agents", cliName, agentsMdFileName)
}

// agentsMdSource renders the final AGENTS.md: the CLI's template if present,
// else the shared one
func agentsMdSource(cliName string, isProxied bool) ([]byte, error) {
	if osutil.Present(cliTmplMdPath(cliName)) {
		return renderAgentsTmpl(cliTmplMdPath(cliName), agentsTmplCtx{isProxied: isProxied})
	}
	return renderAgentsTmpl(userAgentsTmplMdPath(), agentsTmplCtx{isProxied: isProxied, isShared: true})
}

func renderAgentsTmpl(path string, ctx agentsTmplCtx) ([]byte, error) {
	src, err := os.ReadFile(path) // #nosec G304 -- the template's own path
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("agents").
		Delims("<!--{{", "}}-->"). // the actions read as comments in the doc's markdown
		Funcs(ctx.funcs()).
		Option("missingkey=error").
		Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var b bytes.Buffer
	if err := tmpl.Execute(&b, nil); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return b.Bytes(), nil
}

// agentsTmplCtx carries the AGENTS doc template's funcs. isShared marks the shared
// doc itself, where sharedAgentsMd would recurse and so errors
type agentsTmplCtx struct {
	isProxied bool
	isShared  bool
}

var (
	//go:embed admin.md
	adminMd string
	//go:embed admin.proxied.md
	adminProxiedMd string
)

func (c agentsTmplCtx) funcs() template.FuncMap {
	return template.FuncMap{
		"adminSection": func() string { return adminMd },
		"proxySection": func() string { return adminProxiedMd },
		"IsProxied":    func() bool { return c.isProxied },
		"sharedAgentsMd": func() (string, error) {
			if c.isShared {
				return "", errors.New("sharedAgentsMd is only available in a CLI's " + agentsTmplMdFileName)
			}
			body, err := renderAgentsTmpl(userAgentsTmplMdPath(), agentsTmplCtx{isProxied: c.isProxied, isShared: true})
			return string(body), err
		},
	}
}

const (
	agentsMdFileName     = "AGENTS.md"
	agentsTmplMdFileName = "AGENTS.tmpl.md"
)

// realCliFile is the CLI's AGENTS doc: <UserConfigDir>/AGENTS.md
func realCliFile(cliName string) string {
	return filepath.Join(cli.UserConfigDir(cliName), agentsMdFileName)
}

// cliTmplMdPath is the CLI's AGENTS doc template: <UserConfigDir>/AGENTS.tmpl.md
func cliTmplMdPath(cliName string) string {
	return filepath.Join(cli.UserConfigDir(cliName), agentsTmplMdFileName)
}

// userAgentsTmplMdPath is the shared AGENTS doc template: ~/.ccbox/AGENTS.tmpl.md
func userAgentsTmplMdPath() string { return filepath.Join(userdir.Dir(), agentsTmplMdFileName) }
