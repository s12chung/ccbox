package dmap

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/pkg/cli"
)

func TestBuildMap_Options(t *testing.T) {
	options := NewBuildMap("tag", false).Options()
	assert.Equal(t, "tag", options.Tag)
	assert.Equal(t, baseTarget, options.Target)
	assert.Equal(t, map[string]string{dataBindDirsArgKey: dataBindDirs(cli.All())}, options.BuildArgs)
}

func TestBuildMap_Options_VNC(t *testing.T) {
	options := NewBuildMap("tag", true).Options()
	assert.Equal(t, "tag-vnc", options.Tag)
	assert.Equal(t, vncTarget, options.Target)
}

func TestDataBindDirs(t *testing.T) {
	t.Run("joins every cli's data-bind parents under the container home, sorted and deduped", func(t *testing.T) {
		clis := []cli.CLI{
			{PkgInfo: pkginfo.PkgInfo{Name: "opencode"}, DataBinds: map[string]*string{
				".local/share/opencode/auth.json":     new("{}"), // file with seed content
				".local/share/opencode/sessions.json": nil,       // dir: dedupes the parent
			}},
			{PkgInfo: pkginfo.PkgInfo{Name: "mycli"}, DataBinds: map[string]*string{
				".local/share/mycli/store": nil, // dir despite the extension
			}},
		}

		assert.Equal(t,
			"/home/ccbox/.local/share/mycli /home/ccbox/.local/share/opencode",
			dataBindDirs(clis))
	})

	t.Run("skips keys whose parent is home (direct children)", func(t *testing.T) {
		clis := []cli.CLI{
			{PkgInfo: pkginfo.PkgInfo{Name: "mycli"}, DataBinds: map[string]*string{
				".mycli-store": nil, // dir bind at a direct child of home
			}},
		}

		assert.Empty(t, dataBindDirs(clis))
	})

	t.Run("empty when no cli has data binds", func(t *testing.T) {
		assert.Empty(t, dataBindDirs([]cli.CLI{{PkgInfo: pkginfo.PkgInfo{Name: "mycli"}}}))
	})
}
