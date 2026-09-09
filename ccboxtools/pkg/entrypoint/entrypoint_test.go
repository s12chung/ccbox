package entrypoint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecArgv(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		wantErr string
	}{
		{"empty", nil, "no command to exec"},
		{"missing binary", []string{"ccbox-definitely-not-a-binary"}, "exec ccbox-definitely-not-a-binary"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := execArgv(tt.argv)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestCurrentUserName_MatchesOs(t *testing.T) {
	// the test runs as some real user; currentUserName must resolve it, not fail closed
	assert.NotEmpty(t, currentUserName())
}
