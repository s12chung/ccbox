package share

import (
	_ "embed"
	"errors"
	"os"
	"path"
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/fsync"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/sharer"
)

// SafeSeedAgentsMd seeds the shared AGENTS docs when missing: the ccbox-admin variant empty
// and the README.md explainer; existing ones are never touched
func SafeSeedAgentsMd() error {
	if ioutil.Present(userAgentsMdPath()) {
		return nil
	}
	if err := fsync.File(userAgentsAdminMdPath(), ""); err != nil {
		if !errors.Is(err, fsync.ErrExists) {
			return err
		}
		return nil
	}
	if err := fsync.File(userAgentsReadmeMdPath(), agentsReadmeMd); err != nil && !errors.Is(err, fsync.ErrExists) {
		return err
	}
	return nil
}

// AgentsMd returns the ShareBinder for CLI's AGENTS doc. Real copy is at realCliFile
// as part of the CLIConfigDir's bind (hence ShareBind.HostPath = ""). If real copy
// doesn't exist, a symlinked copy is at AgentsMdScratchMount's path and that scratch
// is bound (with the ShareBind.HostPath set and bound to BindPath)
func AgentsMd(cliName string) sharer.ShareBinder {
	mount := AgentsMdScratchMount(cliName)
	return sharer.ShareBinder{
		Share: sharer.Share{
			Name:     "agents",
			RealPath: realCliFile(cliName),
			Content:  sharer.FileScratch{Source: func() ([]byte, error) { return agentsMdSource(cliName) }},
			Sync:     sharer.SymlinkScratch{Mount: mount},
		},
		BindPath: mount,
	}
}

// AgentsMdScratchMount is the CLI's scratch copy's container path:
// <ContainerHome>/.ccbox/tmp/agents/<cli_name>/AGENTS.md
func AgentsMdScratchMount(cliName string) string {
	return path.Join(projectcfg.ContainerHome, ".ccbox", "tmp", "agents", cliName, AgentsMdFileName)
}

// agentsMdSource is the final AGENTS.md, symlinked from the scratch at
// AgentsMdScratchMount. In this priority order: the CLI's ccbox-admin variant, the
// shared doc, then the shared doc's ccbox-admin variant
func agentsMdSource(cliName string) ([]byte, error) {
	if ioutil.Present(cliAdminPath(cliName)) {
		return readAgentsMd(cliAdminPath(cliName))
	}
	if ioutil.Present(userAgentsMdPath()) {
		return os.ReadFile(userAgentsMdPath()) // #nosec G304 -- the shared doc's own path
	}
	return readAgentsMd(userAgentsAdminMdPath())
}

//go:embed admin.md
var adminMd string

//go:embed AGENTS.README.md
var agentsReadmeMd string

const (
	// AgentsMdFileName is the shared AGENTS doc's file name, joined into the scratch
	// file's host and container paths
	AgentsMdFileName      = "AGENTS.md"
	agentsAdminMdFileName = "AGENTS.ccbox-admin.md"

	userAgentsMdFileName       = "AGENTS.user.md"
	userAgentsAdminMdFileName  = "AGENTS.user.ccbox-admin.md"
	userAgentsReadmeMdFileName = "AGENTS.README.md"
)

// readAgentsMd reads a ccbox-admin variant at path, with the embedded admin.md prepended
func readAgentsMd(path string) ([]byte, error) {
	body, err := os.ReadFile(path) // #nosec G304 -- the ccbox-admin variant's own path
	if err != nil {
		return nil, err
	}
	return append([]byte(adminMd), body...), nil
}

// realCliFile is the CLI's AGENTS doc: <UserCLIConfigDir>/AGENTS.md
func realCliFile(cliName string) string {
	return filepath.Join(harness.UserCLIConfigDir(cliName), AgentsMdFileName)
}

// cliAdminPath is the CLI's AGENTS doc's ccbox-admin variant: <UserCLIConfigDir>/AGENTS.ccbox-admin.md
func cliAdminPath(cliName string) string {
	return filepath.Join(harness.UserCLIConfigDir(cliName), agentsAdminMdFileName)
}

// userAgentsMdPath is the shared AGENTS doc's host path: ~/.ccbox/AGENTS.user.md
func userAgentsMdPath() string { return filepath.Join(userdir.Dir(), userAgentsMdFileName) }

// userAgentsAdminMdPath is the shared AGENTS doc's ccbox-admin variant: ~/.ccbox/AGENTS.user.ccbox-admin.md
func userAgentsAdminMdPath() string { return filepath.Join(userdir.Dir(), userAgentsAdminMdFileName) }

// userAgentsReadmeMdPath is the shared AGENTS docs' explainer: ~/.ccbox/AGENTS.README.md
func userAgentsReadmeMdPath() string { return filepath.Join(userdir.Dir(), userAgentsReadmeMdFileName) }
