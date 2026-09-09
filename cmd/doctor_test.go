package cmd

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateTools(t *testing.T) {
	embedded := []byte("embedded bytes")

	tests := []struct {
		name       string
		embedded   []byte
		rebuilt    []byte
		rebuildErr error
		wantErr    string
	}{
		{"fresh", embedded, embedded, nil, ""},
		{"stale", embedded, []byte("fresh bytes"), nil, "run make"},
		{"rebuild error passthrough", embedded, nil, errors.New("compile boom"), "compile boom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTools(tt.embedded, "amd64", func(string) ([]byte, error) {
				return tt.rebuilt, tt.rebuildErr
			})

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestToolsGoarch(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{"from make env", "mips64", "mips64"},
		{"falls back to go env", "", runtime.GOARCH}, // unset → the bare-run fallback
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GOARCH", tt.env)

			got, err := toolsGoarch(context.Background())

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
