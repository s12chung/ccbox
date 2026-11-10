package install

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

func TestFromEnv_NoopWithoutPkginfo(t *testing.T) {
	t.Setenv(pkginfo.EnvVar, "")
	require.NoError(t, FromEnv(t.Context()))
}

func TestFromEnv_BadJSON(t *testing.T) {
	t.Setenv(pkginfo.EnvVar, `{"name":`)

	err := FromEnv(t.Context())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "pkginfo: parse")
}
