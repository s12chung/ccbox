package dmap

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/models/app"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/guiapp"
	"github.com/s12chung/ccbox/pkg/util/slug"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
)

func TestBuildMap_Options(t *testing.T) {
	options := NewBuildMap("tag", t.TempDir(), false, false).Options()
	assert.Equal(t, "tag", options.Tag)
	assert.Equal(t, "base", options.Target)
	assert.Equal(t, map[string]string{dataBindDirsArgKey: strings.Join(dataBindDirs(cli.All(), guiapp.Apps), " ")}, options.BuildArgs)
}

func TestBuildMap_Options_VNC(t *testing.T) {
	options := NewBuildMap("tag", t.TempDir(), false, true).Options()
	assert.Equal(t, "tag-vnc", options.Tag)
	assert.Equal(t, "vnc", options.Target)
}

func TestBuildMap_Options_TagByProject(t *testing.T) {
	projectDir := t.TempDir()
	options := NewBuildMap("tag", projectDir, true, false).Options()
	assert.Equal(t, "tag"+slug.Path(projectDir), options.Tag)
}

func TestDataBindDirs(t *testing.T) {
	t.Run("lists every app's data-bind dirs, sorted", func(t *testing.T) {
		clis := []cli.CLI{
			{PkgInfo: pkginfo.PkgInfo{Name: "opencode"}, Setup: app.Setup{DataBinds: map[string]*string{
				".local/share/opencode/auth.json":  new("{}"), // standalone file: its parent is implied by the dir bind below
				".local/share/opencode/a_dir.json": nil,       // dir bind: lists itself, implying the parent above
			}}},
			{PkgInfo: pkginfo.PkgInfo{Name: "mycli"}, Setup: app.Setup{DataBinds: map[string]*string{
				".local/share/mycli/store": nil, // dir despite the extension
			}}},
		}

		assert.Equal(t,
			[]string{"/home/ccbox/.local/share/mycli/store", "/home/ccbox/.local/share/opencode/a_dir.json", "/home/ccbox/.zcode/v2"},
			dataBindDirs(clis, guiapp.Apps))
	})

	t.Run("the gui app's dirs are always in — the desktop variant alone mounts them", func(t *testing.T) {
		clis := []cli.CLI{{PkgInfo: pkginfo.PkgInfo{Name: "mycli"}}}

		assert.Equal(t, []string{"/home/ccbox/.zcode/v2"}, dataBindDirs(clis, guiapp.Apps))
	})

	t.Run("the proxied overlays go in too", func(t *testing.T) {
		clis := []cli.CLI{{PkgInfo: pkginfo.PkgInfo{Name: "mycli"}, Setup: app.Setup{
			DataBinds: map[string]*string{".mycli/store": nil},
			Proxied:   app.Proxied{DataBinds: map[string]*string{".mycli/proxy/store": nil}},
		}}}

		assert.Equal(t, []string{"/home/ccbox/.mycli/proxy/store", "/home/ccbox/.mycli/store", "/home/ccbox/.zcode/v2"}, dataBindDirs(clis, guiapp.Apps))
	})

	t.Run("skips home-root file binds — home is already there", func(t *testing.T) {
		clis := []cli.CLI{{PkgInfo: pkginfo.PkgInfo{Name: "mycli"}, Setup: app.Setup{DataBinds: map[string]*string{
			".mycli-store": new("{}"), // home-root file: home is its parent
		}}}}

		assert.Equal(t, []string{"/home/ccbox/.zcode/v2"}, dataBindDirs(clis, guiapp.Apps),
			"the home-root file adds no dir — only the gui app's stands")
	})
}
