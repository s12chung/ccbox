package ioutil

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
)

// Expander guards a source file's expansion: a generated file, stale when the source's
// sha256, recorded beside it, no longer matches the source on disk
type Expander struct {
	Source    string // the source file the expansion is generated from
	Expansion string // the generated file's path
}

// IsStale reports whether the expansion is missing or was generated from a different
// source. The sha256 file may be committed by hand, so surrounding whitespace still
// matches.
func (e Expander) IsStale() (bool, error) {
	if Missing(e.Expansion) || Missing(e.SHA256Path()) {
		return true, nil
	}
	sourceSum, err := e.sha256()
	if err != nil {
		return false, err
	}
	sum, err := os.ReadFile(e.SHA256Path()) // #nosec G304 -- the guard's own path
	if err != nil {
		return false, err
	}
	return !strings.EqualFold(strings.TrimSpace(string(sum)), sourceSum), nil
}

// Expand atomically writes the expansion, then its matching sha256: a crash between
// the writes leaves a stale pair, never a wrongly fresh one
func (e Expander) Expand(expansion []byte) error {
	if err := AtomicWriteFile(e.Expansion, expansion); err != nil {
		return err
	}
	sum, err := e.sha256()
	if err != nil {
		return err
	}
	return AtomicWriteFile(e.SHA256Path(), []byte(sum))
}

// sha256 is Source content's hex sha256
func (e Expander) sha256() (string, error) {
	body, err := os.ReadFile(e.Source) // #nosec G304 -- the guard's own source path
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

// SHA256Path is the guard's path, beside Source: its name carries Source's sha256
func (e Expander) SHA256Path() string { return e.Source + ".sha256" }
