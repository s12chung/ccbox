package dmap

import (
	"path"
	"slices"
	"strings"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/models/cli"
	"github.com/s12chung/ccbox/pkg/models/guiapp"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/slug"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
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

// dataBindDirsArgKey is the image build-arg naming the apps' data-bind parent dirs (Dockerfile)
const dataBindDirsArgKey = "data_bind_dirs"

// Options renders the image build's docker.BuildOptions
func (bm *BuildMap) Options() docker.BuildOptions {
	return docker.BuildOptions{
		Tag:    bm.tag + tagSuffix(bm.projectDir, bm.tagByProject, bm.serveVNC),
		Target: buildTarget(bm.serveVNC),
		BuildArgs: map[string]string{
			dataBindDirsArgKey: strings.Join(dataBindDirs(cli.All(), guiapp.Apps), " "),
		},
	}
}

func tagSuffix(projectDir string, tagByProject, serveVNC bool) string {
	suffix := ""
	if serveVNC {
		suffix = "-vnc" // the image doesn't vary on the egress wall
	}
	if tagByProject {
		suffix = slug.Path(projectDir) + suffix // slugs to its own leading dash — no separator
	}
	return suffix
}

// buildTarget returns the Dockerfile stage to build for the vnc mode
func buildTarget(serveVNC bool) string {
	if serveVNC {
		return "vnc" // desktop with vnc
	}
	return "base" // the headless devbox
}

// dataBindDirs lists every app's DataBindContainerDirs under the container home for
// dataBindDirsArgKey
func dataBindDirs(clis []cli.CLI, apps map[string]guiapp.GUIApp) []string {
	dirs := make([]string, 0, len(clis)+len(apps))
	// with proxy, more container dirs and stays consistent
	const noProxy = false
	for _, c := range clis {
		dirs = append(dirs, c.Setup.DataBindContainerDirs(noProxy)...)
	}
	for _, guiApp := range apps {
		dirs = append(dirs, guiApp.Setup.DataBindContainerDirs(noProxy)...)
	}
	for i, dir := range dirs {
		dirs[i] = path.Join(pkginfo.ContainerHome, dir)
	}
	slices.Sort(dirs)
	return osutil.PruneImplied(dirs)
}
