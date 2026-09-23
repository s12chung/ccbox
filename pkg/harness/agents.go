package harness

import (
	"bytes"
	_ "embed"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
)

// SafeSeedAgentsMd lays the shared AGENTS docs when missing: the ccbox-admin variant empty
// and the README.md explainer; existing ones are never touched
func SafeSeedAgentsMd() error {
	if ioutil.Present(userAgentsMdPath()) {
		return nil
	}
	if err := seed.File(userAgentsAdminMdPath(), ""); err != nil {
		if !errors.Is(err, seed.ErrExists) {
			return err
		}
		return nil
	}
	if err := seed.File(userAgentsReadmeMdPath(), agentsReadmeMd); err != nil && !errors.Is(err, seed.ErrExists) {
		return err
	}
	return nil
}

// AgentsMdShare shares the AGENTS doc across CLIs: the original is bound via a scratch copy,
// and changes are preserved into the cliFile. The bound doc is the share's source (see source).
type AgentsMdShare struct {
	// CLI is the run's CLI
	CLI CLI

	// ScratchMountDir is the scratch dir's container path; the cliFile symlink points into it
	ScratchMountDir string
}

var noop = func() error { return nil }

// Begin shares the AGENTS doc with a CLI that has no cliFile: it returns the scratch dir to
// bind and a cleanup settling changes into the cliFile.
func (s AgentsMdShare) Begin() (string, func() error, error) {
	if err := s.clean(); err != nil { // clean a crashed run's leftover symlink and scratch
		return "", noop, err
	}
	if s.definedCliFileExists() {
		return "", noop, nil
	}

	body, err := s.source()
	if err != nil {
		return "", noop, err // no source doc: the bind errors
	}
	if err := ioutil.SafeWriteFile(s.scratchFilePath(), body); err != nil {
		return "", noop, err
	}
	if err := ioutil.SafeSymlink(s.cliFile(), s.cliFileTarget()); err != nil {
		return "", noop, err
	}
	return s.cliTmpPath(), s.clean, nil
}

func (s AgentsMdShare) clean() error {
	// drop the laid symlink first
	if ioutil.IsSymlinkTo(s.cliFile(), s.cliFileTarget()) {
		if err := os.Remove(s.cliFile()); err != nil {
			return err
		}
	}

	scratch := s.scratchFilePath()
	if ioutil.Missing(scratch) {
		return nil
	}

	scratchBody, changed, err := s.scratchDiff()
	if err != nil {
		return err
	}
	if changed {
		if err := s.promote(scratchBody); err != nil {
			return err
		}
	}
	return os.Remove(scratch)
}

// scratchDiff reads the scratch and compares it to the source doc
func (s AgentsMdShare) scratchDiff() ([]byte, bool, error) {
	scratchBody, err := os.ReadFile(s.scratchFilePath()) // #nosec G304 -- the run's own scratch copy
	if err != nil {
		return nil, false, err
	}
	srcBody, err := s.source()
	if err != nil {
		return nil, false, err
	}
	return scratchBody, !bytes.Equal(scratchBody, srcBody), nil
}

// source is the doc the scratch binds, most specific first: the CLI's ccbox-admin variant,
// the shared doc, then the shared doc's ccbox-admin variant; it errors when none exist.
func (s AgentsMdShare) source() ([]byte, error) {
	if ioutil.Present(s.cliAdminPath()) {
		return readAgentsMd(s.cliAdminPath(), true)
	}
	if ioutil.Present(userAgentsMdPath()) {
		return os.ReadFile(userAgentsMdPath()) // #nosec G304 -- the shared doc's own path
	}
	return readAgentsMd(userAgentsAdminMdPath(), true)
}

// readAgentsMd reads body at path, with the embedded admin.md prepended to ccbox-admin variants
func readAgentsMd(path string, admin bool) ([]byte, error) {
	body, err := os.ReadFile(path) // #nosec G304 -- the ccbox-admin variant's own path
	if err != nil {
		return nil, err
	}
	if !admin {
		return body, nil
	}
	return append([]byte(adminMd), body...), nil
}

// promote writes body to the cliFile, logging the set
func (s AgentsMdShare) promote(body []byte) error {
	cliFile := s.cliFile()
	if err := ioutil.SafeWriteFile(cliFile, body); err != nil {
		return err
	}
	log.Infof("set %s", userdir.Tilde(cliFile))
	return nil
}

// cliTmpPath is the scratch file's dir: ~/.ccbox/tmp/<cli_name>
func (s AgentsMdShare) cliTmpPath() string { return filepath.Join(userdir.Tmp(), s.CLI.Name) }

// scratchFilePath is the path of the shared doc: ~/.ccbox/tmp/<cli_name>/AGENTS.md
func (s AgentsMdShare) scratchFilePath() string {
	return filepath.Join(s.cliTmpPath(), agentsMdFileName)
}

//go:embed admin.md
var adminMd string

//go:embed AGENTS.README.md
var agentsReadmeMd string

const (
	agentsMdFileName      = "AGENTS.md"
	agentsAdminMdFileName = "AGENTS.ccbox-admin.md"

	userAgentsMdFileName       = "AGENTS.user.md"
	userAgentsAdminMdFileName  = "AGENTS.user.ccbox-admin.md"
	userAgentsReadmeMdFileName = "AGENTS.README.md"
)

// cliFile is the CLI's AGENTS doc: ~/.ccbox/<cli_name>/AGENTS.md
func (s AgentsMdShare) cliFile() string {
	return filepath.Join(userdir.Dir(), s.CLI.Name, agentsMdFileName)
}

// cliFileTarget is the cliFile symlink's target
func (s AgentsMdShare) cliFileTarget() string { return path.Join(s.ScratchMountDir, agentsMdFileName) }

// definedCliFileExists reports whether the CLI's own doc exists at the cliFile
func (s AgentsMdShare) definedCliFileExists() bool {
	info, err := os.Lstat(s.cliFile())
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false // no doc of its own
	case err != nil:
		return true // unreadable (e.g. a directory): don't touch it
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return true
	}
	target, err := os.Readlink(s.cliFile())
	return err != nil || target != s.cliFileTarget() // unreadable/foreign: leave it be
}

// cliAdminPath is the CLI's AGENTS doc's ccbox-admin variant: ~/.ccbox/<cli_name>/AGENTS.ccbox-admin.md
func (s AgentsMdShare) cliAdminPath() string {
	return filepath.Join(userdir.Dir(), s.CLI.Name, agentsAdminMdFileName)
}

// userAgentsMdPath is the shared AGENTS doc's host path: ~/.ccbox/AGENTS.user.md
func userAgentsMdPath() string { return filepath.Join(userdir.Dir(), userAgentsMdFileName) }

// userAgentsAdminMdPath is the shared AGENTS doc's ccbox-admin variant: ~/.ccbox/AGENTS.user.ccbox-admin.md
func userAgentsAdminMdPath() string { return filepath.Join(userdir.Dir(), userAgentsAdminMdFileName) }

// userAgentsReadmeMdPath is the shared AGENTS docs' explainer: ~/.ccbox/AGENTS.README.md
func userAgentsReadmeMdPath() string { return filepath.Join(userdir.Dir(), userAgentsReadmeMdFileName) }
