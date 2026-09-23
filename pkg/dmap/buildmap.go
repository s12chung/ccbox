package dmap

import (
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/harness"
)

// dataBindDirsArgKey is the image build-arg naming the per-CLI data-bind parent dirs (Dockerfile)
const dataBindDirsArgKey = "data_bind_dirs"

// BuildMap maps the build command's inputs to the docker pkg build options
type BuildMap struct {
	tag string
}

// NewBuildMap returns a new BuildMap
func NewBuildMap(tag string) *BuildMap { return &BuildMap{tag: tag} }

// Options renders the image build's docker.BuildOptions
func (bm *BuildMap) Options() docker.BuildOptions {
	return docker.BuildOptions{
		Tag:       bm.tag,
		BuildArgs: map[string]string{dataBindDirsArgKey: dataBindDirs(harness.All())},
	}
}

// dataBindDirs converts cli.DataBinds to dataBindDirsArgKey's value
// a list of mount points to mkdir -p in the Dockerfile
// so the directories are ccbox user owned for mounts (otherwise root owned)
func dataBindDirs(clis []harness.CLI) string {
	dirs := map[string]struct{}{}
	for _, cli := range clis {
		for key := range cli.DataBinds {
			if dir := path.Dir(key); dir != "." {
				dirs[path.Join(containerHome, dir)] = struct{}{}
			}
		}
	}
	return strings.Join(slices.Sorted(maps.Keys(dirs)), " ")
}
