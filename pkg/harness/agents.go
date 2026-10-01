package harness

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/flock"
	"github.com/s12chung/ccbox/pkg/util/fsync"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/klean"
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

// AgentsMdShare shares the AGENTS doc across CLIs: the original is bound via a scratch copy,
// and changes are preserved into the realCliFile. The bound doc is the share's source (see source).
type AgentsMdShare struct {
	// CLI is the run's CLI
	CLI CLI

	// ScratchMountDir is the scratch dir's container path; the realCliFile symlink points into it
	ScratchMountDir string
}

// Begin shares the AGENTS doc with a CLI that has no realCliFile, returning the scratch
// dir to bind and a cleanup settling changes into the realCliFile. Concurrent runs share
// the one scratch: the first run writes it, latecomers bind it as-is, the last out settles it.
func (s AgentsMdShare) Begin() (string, func() error, error) {
	leave, err := s.multiflock().Join(s.verifyShared, s.initialShare)
	clean := klean.SwallowErr(klean.NewQueue(leave, s.clean).Run, flock.ErrNotLast)
	if err != nil {
		return "", clean, err
	}
	if s.realCliFileExists() {
		return "", clean, nil // "" indicates no mount, the file is already in the realCliFile()
	}
	return s.cliScratchDir(), clean, nil
}

// verifyShared verifies the share a live entry implies is present: the scratch file, or
// the realCliFile when the first holder bound the CLI's own doc directly
func (s AgentsMdShare) verifyShared() error {
	if ioutil.Missing(s.scratchFilePath()) && !s.realCliFileExists() {
		return fmt.Errorf("harness: live run's share missing: neither %s nor %s", userdir.Tilde(s.scratchFilePath()), userdir.Tilde(s.realCliFile()))
	}
	return nil
}

func (s AgentsMdShare) initialShare() error {
	if err := s.clean(); err != nil { // clean a crashed run's leftover symlink and scratch
		return err
	}
	if s.realCliFileExists() { // the CLI's own doc is the share: bound directly, no scratch written
		return nil
	}

	body, err := s.source()
	if err != nil {
		return err // no source doc: the bind errors
	}
	if err := ioutil.SafeWriteFile(s.baseFilePath(), body); err != nil {
		return err
	}
	if err := ioutil.SafeWriteFile(s.scratchFilePath(), body); err != nil {
		return err
	}
	return ioutil.SafeSymlink(s.realCliFile(), s.cliFileTarget())
}

func (s AgentsMdShare) clean() error {
	// drop ccbox's symlink first
	if ioutil.IsSymlinkTo(s.realCliFile(), s.cliFileTarget()) {
		if err := os.Remove(s.realCliFile()); err != nil {
			return err
		}
	}

	scratch := s.scratchFilePath()
	if ioutil.Missing(scratch) {
		return s.removeBase() // a crashed run's lone baseline
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
	if err := os.Remove(scratch); err != nil {
		return err
	}
	return s.removeBase()
}

// removeBase removes the baseline copy when present
func (s AgentsMdShare) removeBase() error {
	if ioutil.Missing(s.baseFilePath()) {
		return nil
	}
	return os.Remove(s.baseFilePath())
}

// scratchDiff reads the scratch and compares it to the body it was bound from
func (s AgentsMdShare) scratchDiff() ([]byte, bool, error) {
	scratchBody, err := os.ReadFile(s.scratchFilePath()) // #nosec G304 -- the run's own scratch copy
	if err != nil {
		return nil, false, err
	}
	base, err := s.base()
	if err != nil {
		return nil, false, err
	}
	return scratchBody, !bytes.Equal(scratchBody, base), nil
}

// base is the body the scratch was bound from: the baseline copy, or the source doc when the
// copy is missing (a past run's leftovers)
func (s AgentsMdShare) base() ([]byte, error) {
	body, err := os.ReadFile(s.baseFilePath()) // #nosec G304 -- the run's own baseline copy
	if !errors.Is(err, fs.ErrNotExist) {
		return body, err
	}
	return s.source()
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

// promote writes body to the realCliFile, logging the set
func (s AgentsMdShare) promote(body []byte) error {
	realCliFile := s.realCliFile()
	if err := ioutil.SafeWriteFile(realCliFile, body); err != nil {
		return err
	}
	log.Infof("set %s", userdir.Tilde(realCliFile))
	return nil
}

// cliScratchDir is the scratch file's dir: ~/.ccbox/tmp/<cli_name>
func (s AgentsMdShare) cliScratchDir() string { return filepath.Join(userdir.Tmp(), s.CLI.Name) }

// multiflock tracks the CLI's live runs: ~/.ccbox/tmp/runs/agents/<cli_name>
func (s AgentsMdShare) multiflock() flock.MultiFlock {
	return flock.MultiFlock{Dir: filepath.Join(userdir.Runs(), "agents", s.CLI.Name)}
}

// scratchFilePath is the path of the shared doc: ~/.ccbox/tmp/<cli_name>/AGENTS.md
func (s AgentsMdShare) scratchFilePath() string {
	return filepath.Join(s.cliScratchDir(), agentsMdFileName)
}

// baseFilePath is the baseline copy the scratch is diffed against. It sits outside the bound
// scratch dir, so the container can't touch it: ~/.ccbox/tmp/<cli_name>.AGENTS.md.orig
func (s AgentsMdShare) baseFilePath() string {
	return filepath.Join(userdir.Tmp(), s.CLI.Name+"."+agentsMdFileName+".orig")
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

// realCliFile is the CLI's AGENTS doc: ~/.ccbox/<cli_name>/AGENTS.md
func (s AgentsMdShare) realCliFile() string {
	return filepath.Join(userdir.Dir(), s.CLI.Name, agentsMdFileName)
}

// cliFileTarget is the realCliFile symlink's target
func (s AgentsMdShare) cliFileTarget() string { return path.Join(s.ScratchMountDir, agentsMdFileName) }

// realCliFileExists reports whether the CLI's own doc exists at the realCliFile
func (s AgentsMdShare) realCliFileExists() bool {
	info, err := os.Lstat(s.realCliFile())
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false // no doc of its own
	case err != nil:
		return true // unreadable (e.g. a directory): don't touch it
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return true
	}
	target, err := os.Readlink(s.realCliFile())
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
