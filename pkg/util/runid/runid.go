// Package runid ids a run's aside files with short random hex
package runid

import (
	"crypto/rand"
	"encoding/hex"
)

// New returns a short random hex id naming one run's aside files
func New() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b) // documented never to fail, Go 1.24 and up
	return hex.EncodeToString(b)
}
