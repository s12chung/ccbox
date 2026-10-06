package install

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

func TestFromEnv_MissingPkginfo(t *testing.T) {
	t.Setenv(pkginfo.EnvVar, "")

	err := FromEnv()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "CLI_PKGINFO is not set")
}

func TestFromEnv_BadJSON(t *testing.T) {
	t.Setenv(pkginfo.EnvVar, `{"name":`)

	err := FromEnv()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pkginfo: parse")
}
