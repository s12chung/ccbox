package harness

import (
	"bytes"
	_ "embed"
	"errors"
	"os"
	"path/filepath"
	"strings"

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
}

var noop = func() error { return nil }

// Begin shares the AGENTS doc with a CLI that has no cliFile: it returns the scratch path to
// bind and a cleanup settling changes into the cliFile.
func (s AgentsMdShare) Begin() (string, func() error, error) {
	if ioutil.Present(s.cliFile()) {
		return "", noop, nil
	}
	if ioutil.Present(s.scratchFile()) {
		return "", noop, s.clean()
	}
	return s.newScratch()
}

// newScratch copies the source doc to the scratch; the returned cleanup settles the changes.
// No source doc: the bind errors.
func (s AgentsMdShare) newScratch() (string, func() error, error) {
	body, err := s.source()
	if err != nil {
		return "", noop, err
	}
	scratch := s.scratchFile()
	if err := os.MkdirAll(filepath.Dir(scratch), ioutil.Dir); err != nil {
		return "", nil, err
	}
	// #nosec G703 -- the scratch copy's own path under ~/.ccbox
	if err := os.WriteFile(scratch, body, ioutil.File); err != nil {
		return "", nil, err
	}
	return scratch, s.clean, nil
}

// clean promotes a scratch diff to the cliFile, then removes the scratch; a
// failed promotion keeps the scratch for the next run.
func (s AgentsMdShare) clean() error {
	scratch := s.scratchFile()
	if ioutil.Missing(scratch) {
		return nil
	}

	body, err := os.ReadFile(scratch) // #nosec G304 -- the run's own scratch copy
	if err != nil {
		return err
	}
	original, err := s.source()
	if err != nil {
		return err
	}
	if !bytes.Equal(body, original) {
		if err := s.promote(body); err != nil {
			return err
		}
	}
	return os.Remove(scratch)
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
	if err := os.MkdirAll(filepath.Dir(cliFile), ioutil.Dir); err != nil {
		return err
	}
	// #nosec G703 -- the cliFile path under ~/.ccbox
	if err := os.WriteFile(cliFile, body, ioutil.File); err != nil {
		return err
	}
	log.Infof("set %s", userdir.Tilde(cliFile))
	return nil
}

// scratchFile is the run's copy of the shared doc: ~/.ccbox/tmp/<cli_name>/<SeedAgentsFilename>
// -- outside the mounted config dirs, so the container never sees it
func (s AgentsMdShare) scratchFile() string {
	return filepath.Join(userdir.Dir(), "tmp", s.CLI.Name, s.CLI.SeedAgentsFilename)
}

//go:embed admin.md
var adminMd string

//go:embed AGENTS.README.md
var agentsReadmeMd string

const (
	userAgentsMdFileName       = "AGENTS.user.md"
	userAgentsReadmeMdFileName = "AGENTS.README.md"
	agentsMdExt                = ".md"
	// ccboxAdminSuffix is the prefix to prepend the ccbox admin context
	ccboxAdminSuffix = ".ccbox-admin.md"
)

// cliFile is the CLI's AGENTS doc: ~/.ccbox/<cli_name>/<SeedAgentsFilename>
func (s AgentsMdShare) cliFile() string {
	return filepath.Join(userdir.Dir(), s.CLI.Name, s.CLI.SeedAgentsFilename)
}

// cliAdminPath is the CLI's AGENTS doc's ccbox-admin variant: ~/.ccbox/<cli_name>/CLAUDE.ccbox-admin.md
func (s AgentsMdShare) cliAdminPath() string {
	return filepath.Join(userdir.Dir(), s.CLI.Name, adminMdFileName(s.CLI.SeedAgentsFilename))
}

// userAgentsMdPath is the shared AGENTS doc's host path: ~/.ccbox/AGENTS.user.md
func userAgentsMdPath() string { return filepath.Join(userdir.Dir(), userAgentsMdFileName) }

// userAgentsAdminMdPath is the shared AGENTS doc's ccbox-admin variant: ~/.ccbox/AGENTS.user.ccbox-admin.md
func userAgentsAdminMdPath() string {
	return filepath.Join(userdir.Dir(), adminMdFileName(userAgentsMdFileName))
}

// userAgentsReadmeMdPath is the shared AGENTS docs' explainer: ~/.ccbox/AGENTS.README.md
func userAgentsReadmeMdPath() string {
	return filepath.Join(userdir.Dir(), userAgentsReadmeMdFileName)
}

// adminMdFileName names a doc's ccbox-admin variant: CLAUDE.md -> CLAUDE.ccbox-admin.md
func adminMdFileName(fileName string) string {
	return strings.TrimSuffix(fileName, agentsMdExt) + ccboxAdminSuffix
}
