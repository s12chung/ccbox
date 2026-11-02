//go:build linux

package seccomputil

import (
	"encoding/json"
	"testing"

	"github.com/moby/profiles/seccomp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// the library gates DefaultProfile to linux: where the call can run, the
// carried render must equal it
func TestDefaultProfile_IsTheLiveCall(t *testing.T) {
	var carried seccomp.Seccomp
	require.NoError(t, json.Unmarshal(seccompDefaultJSON, &carried))

	assert.Equal(t, seccomp.DefaultProfile(), &carried)
}
