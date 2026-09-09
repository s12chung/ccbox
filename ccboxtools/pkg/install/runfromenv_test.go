package install

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

func TestRunFromEnv_MissingPkginfo(t *testing.T) {
	t.Setenv(pkginfo.EnvVar, "")

	err := RunFromEnv()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "CLI_PKGINFO is not set")
}

func TestRunFromEnv_BadJSON(t *testing.T) {
	t.Setenv(pkginfo.EnvVar, `{"name":`)

	err := RunFromEnv()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pkginfo: parse")
}
