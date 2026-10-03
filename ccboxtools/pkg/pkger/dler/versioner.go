package dler

import (
	"strings"
)

// Versioner resolves a version URL endpoint's body to its version.
type Versioner interface {
	// Latest is the body's version.
	Latest(body []byte) (string, error)
}

// TextVersioner reads the body itself as the version.
type TextVersioner struct{}

// Latest is the trimmed body.
func (TextVersioner) Latest(body []byte) (string, error) {
	return strings.TrimSpace(string(body)), nil
}
