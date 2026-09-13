package harness

import (
	"bytes"
	_ "embed"
	"errors"
	"io/fs"
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
	if err := s.clean(); err != nil { // clean leftover scratch and shadow
		return "", noop, err
	}
	if s.definedCliFileExists() {
		return "", noop, nil
	}

	body, err := s.source()
	if err != nil {
		return "", noop, err // no source doc: the bind errors
	}
	scratch := s.scratchFilePath()
	if err := ioutil.SafeWriteFile(scratch, body); err != nil {
		return "", noop, err
	}
	if !ioutil.Present(s.cliFile()) {
		// create shadow file due to mount creating an empty file via. runc
		if err := ioutil.SafeWriteFile(s.cliFile(), []byte(shadowBody)); err != nil {
			return "", noop, err
		}
	}
	return scratch, s.clean, nil
}

// clean deletes the shadow and scratch, promoting the scratch if changed
func (s AgentsMdShare) clean() error {
	// drop the shadow first, like definedCliFileExists(), only remove actual shadow files
	if cliFileBody, err := os.ReadFile(s.cliFile()); err == nil && isShadowBody(cliFileBody) {
		if err := os.Remove(s.cliFile()); err != nil {
			return err
		}
	}

	scratchPath := s.scratchFilePath()
	if ioutil.Missing(scratchPath) {
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
	return os.Remove(scratchPath)
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

// scratchFilePath is the path of the shared doc: ~/.ccbox/tmp/<cli_name>/<SeedAgentsFilename>
func (s AgentsMdShare) scratchFilePath() string {
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

	// shadowBody marks the cliFile as ccbox's temp shadow file
	shadowBody = "ccbox temp file — safe to delete"
)

// cliFile is the CLI's AGENTS doc: ~/.ccbox/<cli_name>/<SeedAgentsFilename>
func (s AgentsMdShare) cliFile() string {
	return filepath.Join(userdir.Dir(), s.CLI.Name, s.CLI.SeedAgentsFilename)
}

// definedCliFileExists reports whether the CLI's own doc exists at the cliFile
func (s AgentsMdShare) definedCliFileExists() bool {
	body, err := os.ReadFile(s.cliFile())
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false // no doc of its own
	case err != nil:
		return true // unreadable (e.g. a directory): don't touch it
	}
	return !isShadowBody(body)
}

// isShadowBody reports whether body is the shadow file's: ccbox's temp body
func isShadowBody(body []byte) bool { return string(body) == shadowBody }

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
