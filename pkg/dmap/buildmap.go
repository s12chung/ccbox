package dmap

import (
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/util/slug"
)

// BuildMap maps the build command's inputs to the docker pkg build options
type BuildMap struct {
	tag          string
	projectDir   string
	tagByProject bool
	serveVNC     bool
}

// NewBuildMap returns a new BuildMap; serveVNC builds the image's desktop variant
func NewBuildMap(tag, projectDir string, tagByProject, serveVNC bool) *BuildMap {
	return &BuildMap{tag: tag, projectDir: projectDir, tagByProject: tagByProject, serveVNC: serveVNC}
}

// dataBindDirsArgKey is the image build-arg naming the per-CLI data-bind parent dirs (Dockerfile)
const dataBindDirsArgKey = "data_bind_dirs"

// Options renders the image build's docker.BuildOptions
func (bm *BuildMap) Options() docker.BuildOptions {
	return docker.BuildOptions{
		Tag:       bm.tag + tagSuffix(bm.projectDir, bm.tagByProject, bm.serveVNC),
		Target:    buildTarget(bm.serveVNC),
		BuildArgs: map[string]string{dataBindDirsArgKey: dataBindDirs(cli.All())},
	}
}

const vncTagSuffix = "-vnc"

func tagSuffix(projectDir string, tagByProject, serveVNC bool) string {
	suffix := ""
	if tagByProject {
		suffix += slug.Path(projectDir)
	}
	if serveVNC {
		suffix += vncTagSuffix
	}
	// marks the tag of a project with its own mise config
	// no separator due to abs path always turns in to dash
	return suffix
}

const (
	baseTarget = "base" // the headless devbox
	vncTarget  = "vnc"  // desktop with vnc
)

// buildTarget returns the Dockerfile stage to build for the vnc mode
func buildTarget(serveVNC bool) string {
	if serveVNC {
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
