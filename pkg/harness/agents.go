package harness

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
)

const userAgentsMdFileName = "AGENTS.user.md"

// UserAgentsMdPath is the shared AGENTS doc's host path: ~/.ccbox/AGENTS.user.md
func UserAgentsMdPath() string { return filepath.Join(userdir.Dir(), userAgentsMdFileName) }

// SafeSeedAgentsMd lays the shared AGENTS doc empty when missing; an existing one is never touched
func SafeSeedAgentsMd() error {
	err := seed.File(UserAgentsMdPath(), "")
	if errors.Is(err, seed.ErrExists) {
		return nil
	}
	return err
}

// AgentsMdShare shares the AGENTS doc at UserAgentsMdPath() across CLIs: the original is
// bound via a scratch copy, and changes are preserved into the cliFile.
type AgentsMdShare struct {
	// CLI is the run's CLI
	CLI CLI
}

// Begin shares the AGENTS doc at UserAgentsMdPath() with a CLI that has no cliFile:
// it returns the scratch path to bind and a cleanup settling changes into the cliFile.
func (s AgentsMdShare) Begin() (string, func() error, error) {
	noop := func() error { return nil }
	if ioutil.Present(s.cliFile()) {
		return "", noop, nil
	}
	if ioutil.Present(s.scratchFile()) {
		return "", noop, s.clean() // crashed run's leftover: settle it, skip the bind
	}
	return s.newScratch()
}

// newScratch copies UserAgentsMdPath() to the scratch; the returned cleanup settles the changes.
func (s AgentsMdShare) newScratch() (string, func() error, error) {
	scratch := s.scratchFile()
	// assumes SafeSeedAgentsMd() is already called
	body, err := os.ReadFile(UserAgentsMdPath()) // #nosec G304 -- the shared doc's own path
	if err != nil {
		return "", nil, err
	}
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
	original, err := os.ReadFile(UserAgentsMdPath()) // #nosec G304 -- the shared doc's own path
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

// cliFile is the CLI's AGENTS doc: ~/.ccbox/<cli_name>/<SeedAgentsFilename>
func (s AgentsMdShare) cliFile() string {
	return filepath.Join(userdir.Dir(), s.CLI.Name, s.CLI.SeedAgentsFilename)
}
