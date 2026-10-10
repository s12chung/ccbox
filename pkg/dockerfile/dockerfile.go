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
	"github.com/s12chung/ccbox/pkg/util/mfs"
)

var (
	//go:embed Dockerfile.tmpl
	tmplBody []byte
	tmpl     = template.Must(template.New("Dockerfile.tmpl").Funcs(regions).Parse(string(tmplBody)))
)

// regionFunc binds a region generator to all runtimes.
func regionFunc(generate func([]runtime.Runtime) string) func() string {
	return func() string { return generate(runtime.All()) }
}

// regions maps the tmpl's action names to their region generators, bound to
// all runtimes.
var regions = template.FuncMap{
	// the generated apt RUN's one continuation row
	"runtimeLibs": regionFunc(runtimeRegion{
		prefix: "        ", sep: " ", suffix: " \\",
		items: func(r runtime.Runtime) []string { return r.AptPkgs },
	}.render),
	// the from-source compile RUN's $buildDeps
	"buildDeps": regionFunc(runtimeRegion{
		sep:   " ",
		items: func(r runtime.Runtime) []string { return r.AptBuildDeps },
	}.render),
	// the user PATH ENV's runtime segment
	"PATHDirs": regionFunc(runtimeRegion{
		sep:   ":",
		items: func(r runtime.Runtime) []string { return r.BinEntries },
	}.render),
	// the runtimes' image env pairs
	"envVars": regionFunc(runtimeRegion{
		sep:   " ",
		items: envVarRows,
	}.render),
	// the generated mkdir RUN's cache volume mountpoints
	"cacheDirs": regionFunc(runtimeRegion{
		sep:   " ",
		items: func(r runtime.Runtime) []string { return r.CacheVolumes },
	}.render),
}

// PatchFS renders the Dockerfile.tmpl and layers it over fsys as the context's
// Dockerfile.
func PatchFS(fsys fs.FS) (fs.FS, error) {
	body, err := render()
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

// render executes the tmpl: each action generates its region from all runtimes.
func render() ([]byte, error) {
	var b bytes.Buffer
	if err := tmpl.Execute(&b, nil); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// runtimeRegion renders all runtimes' items into one region.
type runtimeRegion struct {
	prefix string
	sep    string
	suffix string
	items  func(runtime.Runtime) []string
}

func (rr runtimeRegion) render(runtimes []runtime.Runtime) string {
	all := make([][]string, 0, len(runtimes))
	for _, r := range runtimes {
		all = append(all, rr.items(r))
	}
	return rr.prefix + strings.Join(slices.Concat(all...), rr.sep) + rr.suffix
}

// envVarRows renders one runtime's image env pairs, keys sorted
func envVarRows(r runtime.Runtime) []string {
	pairs := make([]string, 0, len(r.EnvVars))
	// Go map iteration is randomized
	for _, k := range slices.Sorted(maps.Keys(r.EnvVars)) {
		pairs = append(pairs, k+"="+r.EnvVars[k])
	}
	return pairs
}
