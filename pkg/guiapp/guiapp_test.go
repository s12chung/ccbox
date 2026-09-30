package guiapp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

func TestJSON(t *testing.T) {
	body, err := JSON()
	require.NoError(t, err)

	// the container parses it back through firm validation — the host-built
	// config must stay valid there
	info, err := pkginfo.GUIFromJSON(body)
	require.NoError(t, err)
	assert.Equal(t, App, info)
}

func TestApp_WiresLaunch(t *testing.T) {
	assert.NotEmpty(t, App.Args, "the launcher prepends the app's own argv")
	assert.NotEmpty(t, App.DesktopName, "the session menu entry carries the app's branding")
}
