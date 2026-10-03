package dmap

import (
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/s12chung/ccbox/pkg/cli"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/projectcfg"
)

// dataBindDirsArgKey is the image build-arg naming the per-CLI data-bind parent dirs (Dockerfile)
const dataBindDirsArgKey = "data_bind_dirs"

// The image's build targets — the Dockerfile's stages. base is the headless
// devbox; vnc layers the desktop (TigerVNC serving XFCE, the session's browser
// wrapper) on top.
const (
	baseTarget = "base"
	vncTarget  = "vnc"
)

// vncTagSuffix marks an image tag as the desktop variant
const vncTagSuffix = "-vnc"

// BuildMap maps the build command's inputs to the docker pkg build options
type BuildMap struct {
	tag string
	vnc bool
}

// NewBuildMap returns a new BuildMap; vnc builds the image's desktop variant
func NewBuildMap(tag string, vnc bool) *BuildMap { return &BuildMap{tag: tag, vnc: vnc} }

// Options renders the image build's docker.BuildOptions
func (bm *BuildMap) Options() docker.BuildOptions {
	return docker.BuildOptions{
		Tag:       variantTag(bm.tag, bm.vnc),
		Target:    buildTarget(bm.vnc),
		BuildArgs: map[string]string{dataBindDirsArgKey: dataBindDirs(cli.All())},
	}
}

// variantTag returns the image tag for the vnc mode: the tag, suffixed for the desktop variant
func variantTag(tag string, vnc bool) string {
	if vnc {
		return tag + vncTagSuffix
	}
	return tag
}

// buildTarget returns the Dockerfile stage to build for the vnc mode
func buildTarget(vnc bool) string {
	if vnc {
		return vncTarget
	}
	return baseTarget
}

// dataBindDirs converts cli.DataBinds to dataBindDirsArgKey's value
// a list of mount points to mkdir -p in the Dockerfile
// so the directories are ccbox user owned for mounts (otherwise root owned)
func dataBindDirs(clis []cli.CLI) string {
	dirs := map[string]struct{}{}
	for _, cli := range clis {
		for key := range cli.DataBinds {
			if dir := path.Dir(key); dir != "." {
				dirs[path.Join(projectcfg.ContainerHome, dir)] = struct{}{}
			}
		}
	}
	return strings.Join(slices.Sorted(maps.Keys(dirs)), " ")
}
