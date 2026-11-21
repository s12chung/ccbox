// Package verify holds the pinned-hash check for streamed downloads:
// Sha512Reader hashes the body while it reads through, failing the read on
// mismatch — no second pass or full-body buffer.
package verify

import (
	"bytes"
	sha512pkg "crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
)

// Sha512Reader hashes the body while reading; on the underlying EOF it fails
// the final read — not Close, whose error callers drop — when the body's
// sha512 mismatches the pinned hash in its encoding.
type Sha512Reader struct {
	io.ReadCloser

	Sha512 *Sha512

	url  string
	hash hash.Hash
}

// Sha512 is a pinned hash: the hash's string and its encoding.
type Sha512 struct {
	Hash     string
	Encoding string
}

// NewSha512Reader wraps r when a hash is pinned; r passes through unwrapped
// when none is.
func NewSha512Reader(r io.ReadCloser, url string, sha512 *Sha512) io.ReadCloser {
	if sha512 == nil {
		return r
	}
	return &Sha512Reader{ReadCloser: r, Sha512: sha512, url: url, hash: sha512pkg.New()}
}

// Read hashes into p and fails the final read on a hash mismatch.
func (r *Sha512Reader) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if n > 0 {
		r.hash.Write(p[:n]) //nolint:errcheck // hash.Hash's Write never errors
	}
	if errors.Is(err, io.EOF) {
		if decoded := DecodeSHA(*r.Sha512); !bytes.Equal(decoded, r.hash.Sum(nil)) {
			return n, fmt.Errorf("%s: sha512 mismatch: want %s, got %s", r.url, r.Sha512.Hash, EncodeSum(r.hash.Sum(nil), r.Sha512.Encoding))
		}
	}
	return n, err
}

// DecodeSHA decodes a pinned sha512 in its encoding — "base64" (std) or
// "hex"; anything else decodes to nil.
func DecodeSHA(sha Sha512) []byte {
	var raw []byte
	var err error
	switch sha.Encoding {
	case "base64":
		raw, err = base64.StdEncoding.DecodeString(sha.Hash)
	case "hex":
		raw, err = hex.DecodeString(sha.Hash)
	}
	if err != nil || len(raw) != sha512pkg.Size {
		return nil
	}
	return raw
}

// EncodeSum encodes a sha512 sum in the given encoding — the mismatch error's
// got side, mirroring the pinned hash's.
func EncodeSum(sum []byte, encoding string) string {
	if encoding == "hex" {
		return hex.EncodeToString(sum)
	}
	return base64.StdEncoding.EncodeToString(sum)
}
