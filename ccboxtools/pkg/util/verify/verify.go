// Package verify holds the pinned-hash check for streamed downloads:
// Sha512Reader hashes the body while it reads through, failing the read on
// mismatch — no second pass or full-body buffer.
package verify

import (
	sha512pkg "crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"io"
)

// Sha512Reader hashes the body while reading; on the underlying EOF it fails
// the final read — not Close, whose error callers drop — when the body's
// sha512 mismatches the pinned one (base64 std, the one pinned encoding).
type Sha512Reader struct {
	io.ReadCloser

	hash   hash.Hash
	sha512 string
	url    string
}

// NewSha512Reader wraps r when sha512 pins a hash; r passes through unwrapped
// when none is pinned.
func NewSha512Reader(r io.ReadCloser, url, sha512 string) io.ReadCloser {
	if sha512 == "" {
		return r
	}
	return &Sha512Reader{ReadCloser: r, hash: sha512pkg.New(), sha512: sha512, url: url}
}

// Read hashes into p and fails the final read on a hash mismatch.
func (r *Sha512Reader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		r.hash.Write(p[:n]) //nolint:errcheck // hash.Hash's Write never errors
	}
	if errors.Is(err, io.EOF) {
		if got := base64.StdEncoding.EncodeToString(r.hash.Sum(nil)); got != r.sha512 {
			return n, fmt.Errorf("%s: sha512 mismatch: want %s, got %s", r.url, r.sha512, got)
		}
	}
	return n, err
}
