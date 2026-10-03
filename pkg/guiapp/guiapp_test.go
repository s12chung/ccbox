package guiapp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

func TestFor(t *testing.T) {
	app, err := For(App.Name)
	require.NoError(t, err)
	assert.Equal(t, App, app)

	assert.Contains(t, Names(), App.Name)

	_, err = For("emacs")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not one of [zcode]")
}

func TestAllowDomains(t *testing.T) {
	assert.Equal(t, App.AllowDomains, AllowDomains(App.Name))

	for _, name := range []string{"", "emacs"} {
		assert.Emptyf(t, AllowDomains(name), "%q names no app to download", name)
	}
}

func TestApp_WiresLaunch(t *testing.T) {
	assert.NotEmpty(t, App.Args, "the launcher prepends the app's own argv")
	assert.NotEmpty(t, App.DesktopName, "the session menu entry carries the app's branding")
}

func TestApp_RoundTripsVNCInfo(t *testing.T) {
	// the container parses the run's VNC_CONFIG through firm validation — the
	// host-built config must stay valid there
	body, err := json.Marshal(pkginfo.VNCInfo{GUIApp: &App.GUIPkgInfo, Config: &pkginfo.VNCConfig{Resolution: "1600x900"}})
	require.NoError(t, err)

	info, err := pkginfo.VNCInfoFromJSON(string(body))
	require.NoError(t, err)
	require.NotNil(t, info.GUIApp)
	assert.Equal(t, App.GUIPkgInfo, *info.GUIApp)
}
