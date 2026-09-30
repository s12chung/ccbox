package verify

import (
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pinSha512 is body's hash in the one pinned encoding.
func pinSha512(body string) string {
	sum := sha512.Sum512([]byte(body))
	return base64.StdEncoding.EncodeToString(sum[:])
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
	assert.Equal(t, rc, NewSha512Reader(rc, "https://example.com/file", ""))
}

func TestSha512Reader_Read(t *testing.T) {
	const url = "https://example.com/file"
	tests := []struct {
		name    string
		body    string
		sha512  string
		wantErr string
	}{
		{
			name:   "match",
			body:   "download body",
			sha512: pinSha512("download body"),
		},
		{
			name:   "empty body",
			body:   "",
			sha512: pinSha512(""),
		},
		{
			name:    "mismatch",
			body:    "tampered",
			sha512:  pinSha512("original"),
			wantErr: url + ": sha512 mismatch: want " + pinSha512("original") + ", got " + pinSha512("tampered"),
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

func TestSha512Reader_UnderlyingError(t *testing.T) {
	r := NewSha512Reader(io.NopCloser(errReader{}), "https://example.com/file", pinSha512("body"))
	_, err := r.Read(make([]byte, 4))
	require.ErrorContains(t, err, "boom")
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("boom") }
