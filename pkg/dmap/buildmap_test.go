package dmap

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/pkg/harness"
)

func TestBuildMap_Options(t *testing.T) {
	options := NewBuildMap("tag").Options()
	assert.Equal(t, "tag", options.Tag)
	assert.Equal(t, map[string]string{dataBindDirsArgKey: dataBindDirs(harness.All())}, options.BuildArgs)
}

func TestDataBindDirs(t *testing.T) {
	t.Run("joins every cli's data-bind parents under containerHome, sorted and deduped", func(t *testing.T) {
		clis := []harness.CLI{
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
		clis := []harness.CLI{
			{PkgInfo: pkginfo.PkgInfo{Name: "mycli"}, DataBinds: map[string]*string{
				".mycli-store": nil, // dir bind at a direct child of home
			}},
		}

		assert.Empty(t, dataBindDirs(clis))
	})

	t.Run("empty when no cli has data binds", func(t *testing.T) {
		assert.Empty(t, dataBindDirs([]harness.CLI{{PkgInfo: pkginfo.PkgInfo{Name: "mycli"}}}))
	})
}
