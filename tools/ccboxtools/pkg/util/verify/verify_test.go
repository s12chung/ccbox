package verify

import (
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sha512Pin is body's hash, base64 std-encoded.
func sha512Pin(body string) string {
	sum := sha512.Sum512([]byte(body))
	return base64.StdEncoding.EncodeToString(sum[:])
}

// sha512PinHex is body's hash, hex-encoded.
func sha512PinHex(body string) string {
	sum := sha512.Sum512([]byte(body))
	return hex.EncodeToString(sum[:])
}

// readChunks reads through r in size-byte reads, so the hash splits across reads.
func readChunks(r io.Reader, size int) (string, error) {
	var sb strings.Builder
	p := make([]byte, size)
	for {
		n, err := r.Read(p)
		sb.Write(p[:n])
		if errors.Is(err, io.EOF) {
			return sb.String(), nil
		}
		if err != nil {
			return sb.String(), err
		}
	}
}

func TestNewSha512Reader_Unpinned(t *testing.T) {
	rc := io.NopCloser(strings.NewReader("body"))
	assert.Equal(t, rc, NewSha512Reader(rc, "https://example.com/file", nil))
}

func TestSha512Reader_Read(t *testing.T) {
	const url = "https://example.com/file"
	tests := []struct {
		name    string
		sha512  *Sha512
		body    string
		wantErr string
	}{
		{
			name:   "match, base64 hash",
			sha512: &Sha512{Hash: sha512Pin("download body"), Encoding: "base64"},
			body:   "download body",
		},
		{
			name:   "match, hex hash",
			sha512: &Sha512{Hash: sha512PinHex("download body"), Encoding: "hex"},
			body:   "download body",
		},
		{
			name:   "empty body",
			sha512: &Sha512{Hash: sha512Pin(""), Encoding: "base64"},
			body:   "",
		},
		{
			name:    "mismatch, base64 hash",
			sha512:  &Sha512{Hash: sha512Pin("original"), Encoding: "base64"},
			body:    "tampered",
			wantErr: url + ": sha512 mismatch: want " + sha512Pin("original") + ", got " + sha512Pin("tampered"),
		},
		{
			name:    "mismatch, hex hash", // the got side mirrors the hash's encoding
			sha512:  &Sha512{Hash: sha512PinHex("original"), Encoding: "hex"},
			body:    "tampered",
			wantErr: url + ": sha512 mismatch: want " + sha512PinHex("original") + ", got " + sha512PinHex("tampered"),
		},
		{
			name:    "invalid hash",
			sha512:  &Sha512{Hash: "not-a-hash", Encoding: "base64"},
			body:    "body",
			wantErr: url + ": sha512 mismatch: want not-a-hash, got " + sha512Pin("body"),
		},
		{
			name:    "unknown encoding",
			sha512:  &Sha512{Hash: sha512Pin("body"), Encoding: "b64"},
			body:    "body",
			wantErr: url + ": sha512 mismatch: want " + sha512Pin("body") + ", got " + sha512Pin("body"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewSha512Reader(io.NopCloser(strings.NewReader(tt.body)), url, tt.sha512)
			got, err := readChunks(r, 3)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.body, got)
		})
	}
}

func TestDecodeSHA(t *testing.T) {
	sum := make([]byte, sha512.Size)
	tests := []struct {
		sha Sha512
		ok  bool
	}{
		{Sha512{Hash: base64.StdEncoding.EncodeToString(sum), Encoding: "base64"}, true},
		{Sha512{Hash: hex.EncodeToString(sum), Encoding: "hex"}, true},
		{Sha512{Hash: hex.EncodeToString(sum), Encoding: "base64"}, false},                       // hex hash, wrong encoding
		{Sha512{Hash: hex.EncodeToString(make([]byte, sha512.Size256)), Encoding: "hex"}, false}, // hex, wrong size
		{Sha512{}, false}, // no default encoding
	}
	for _, tt := range tests {
		decoded := DecodeSHA(tt.sha)
		if tt.ok {
			assert.Equalf(t, sum, decoded, "%s in %q", tt.sha.Hash, tt.sha.Encoding)
		} else {
			assert.Nilf(t, decoded, "%s in %q", tt.sha.Hash, tt.sha.Encoding)
		}
	}
}

func TestSha512Reader_UnderlyingError(t *testing.T) {
	r := NewSha512Reader(io.NopCloser(errReader{}), "https://example.com/file", &Sha512{Hash: sha512Pin("body"), Encoding: "base64"})
	_, err := r.Read(make([]byte, 4))
	require.ErrorContains(t, err, "boom")
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }
