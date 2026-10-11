// Package dockerfile renders the image's Dockerfile from its embedded
// Dockerfile.tmpl. PatchFS layers the render over the build context.
package dockerfile

import (
	"bytes"
	_ "embed"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"text/template"

	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/prov"
	"github.com/s12chung/ccbox/pkg/util/mfs"
)

var (
	//go:embed Dockerfile.tmpl
	tmplBody []byte
	tmpl     = template.Must(template.New("Dockerfile.tmpl").Parse(string(tmplBody)))
)

// regions is the tmpl's data; its exported methods are the tmpl's actions.
type regions struct{ prov.Resolved }

// RuntimeLibs renders the generated apt RUN's continuation row
func (r regions) RuntimeLibs() string {
	libs := append(r.collect(func(rt runtime.Runtime) []string { return rt.AptPkgs }), r.AptPackages...)
	return "        " + strings.Join(libs, " ") + " \\"
}

// BuildDeps renders the from-source compile RUN's $buildDeps
func (r regions) BuildDeps() string {
	return strings.Join(r.collect(func(rt runtime.Runtime) []string { return rt.AptBuildDeps }), " ")
}

// PATHDirs renders the user PATH ENV's runtime segment
func (r regions) PATHDirs() string {
	return strings.Join(r.collect(func(rt runtime.Runtime) []string { return rt.BinEntries }), ":")
}

// EnvVars renders the runtimes' image env pairs
func (r regions) EnvVars() string {
	var pairs []string
	for _, rt := range r.Runtimes {
		// Go map iteration is randomized
		for _, k := range slices.Sorted(maps.Keys(rt.EnvVars)) {
			pairs = append(pairs, k+"="+rt.EnvVars[k])
		}
	}
	return strings.Join(pairs, " ")
}

// CacheDirs renders the generated mkdir RUN's cache volume mountpoints
func (r regions) CacheDirs() string {
	return strings.Join(r.collect(func(rt runtime.Runtime) []string { return rt.CacheVolumes }), " ")
}

// collect gathers field's items, keeping All()'s order
func (r regions) collect(field func(runtime.Runtime) []string) []string {
	items := make([]string, 0, len(r.Runtimes))
	for _, rt := range r.Runtimes {
		items = append(items, field(rt)...)
	}
	return items
}

// PatchFS renders the Dockerfile.tmpl and layers it over fsys as the context's
// Dockerfile.
func PatchFS(fsys fs.FS, resolved prov.Resolved) (fs.FS, error) {
	body, err := render(resolved)
	if err != nil {
		return nil, err
	}
	merged, err := mfs.NewFS(fsys)
	if err != nil {
		return nil, err
	}
	if err := merged.Merge(mfs.MapFS{"Dockerfile": {Data: body}}); err != nil {
		return nil, err
	}
	return merged, nil
}

// MiseImageRef extracts the tmpl's `FROM <image> AS mise`; the lock container
// must run the same mise the image builds from, so the two can't drift.
func MiseImageRef() (string, error) {
	return mise.ImageRefFromDockerfile(tmplBody)
}

func render(resolved prov.Resolved) ([]byte, error) {
	var b bytes.Buffer
	if err := tmpl.Execute(&b, regions{resolved}); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
