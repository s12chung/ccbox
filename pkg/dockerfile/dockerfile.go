// Package dockerfile renders the image's Dockerfile from its embedded
// Dockerfile.tmpl: the template's actions generate the runtime-owned statements
// from the runtimes (pkg/models/runtime), and PatchFS layers the render into the
// build context at `ccbox build` — the tmpl itself never ships.
package dockerfile

import (
	_ "embed"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"text/template"

	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/util/mfs"
)

//go:embed Dockerfile.tmpl
var tmplBody []byte

// regions generates each template action's content from all runtimes; the tmpl
// must render every one (render hard-errors otherwise). The names are the
// template's action names, and the testdata goldens ride them.
var regions = map[string]func([]runtime.Runtime) string{
	"apt":         aptRegion,
	"miseInstall": miseInstallRegion,
	"runtimeEnv":  runtimeEnvRegion,
	"path":        pathRegion,
	"preCreate":   preCreateRegion,
}

// PatchFS renders the Dockerfile.tmpl and layers it over fsys as the context's
// Dockerfile.
func PatchFS(fsys fs.FS) (fs.FS, error) {
	body, err := render(tmplBody)
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

// Template is the Dockerfile template's body
func Template() []byte { return tmplBody }

// render executes the tmpl body: each action generates its region from all
// runtimes, and a tmpl that never renders a region is a hard error — the tmpl
// and this composer must agree on every region.
func render(body []byte) ([]byte, error) {
	runtimes := runtime.All()
	seen := make(map[string]bool, len(regions))
	funcs := make(template.FuncMap, len(regions))
	for name, generate := range regions {
		funcs[name] = func() string {
			seen[name] = true
			// the tmpl's own newlines space the regions; the generators' trailing one drops
			return strings.TrimSuffix(generate(runtimes), "\n")
		}
	}
	tmpl, err := template.New("Dockerfile.tmpl").Funcs(funcs).Parse(string(body))
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	if err := tmpl.Execute(&b, nil); err != nil {
		return nil, err
	}
	var missing []string
	for _, name := range slices.Sorted(maps.Keys(regions)) {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("dockerfile: the template never renders: %s", strings.Join(missing, ", "))
	}
	return []byte(b.String()), nil
}

// aptRegion generates the base apt RUN: the shared rows every image rides, then
// one row per runtime with AptPkgs.
var aptSharedRows = []struct{ comment, pkgs string }{
	{"build toolchain", "build-essential"},
	{"TLS roots for mise downloads", "ca-certificates"},
	{"dev-workflow CLIs", "git curl less procps pkg-config unzip bind9-dnsutils"},
}

// aptRowComments notes the why for runtimes whose libs need one; the rest render
// the generic "<Name> runtime libs".
var aptRowComments = map[string]string{
	"node": "Node runtime lib (V8 needs libatomic on arm64)",
}

func aptRegion(runtimes []runtime.Runtime) string {
	var b strings.Builder
	b.WriteString("# Each row is a section:\n")
	for _, row := range aptSharedRows {
		b.WriteString("# - " + row.comment + "\n")
	}
	for _, r := range runtimes {
		if len(r.AptPkgs) == 0 {
			continue
		}
		comment, noted := aptRowComments[r.Name]
		if !noted {
			comment = strings.ToUpper(r.Name[:1]) + r.Name[1:] + " runtime libs"
		}
		b.WriteString("# - " + comment + "\n")
	}
	b.WriteString("RUN apt-get update && apt-get install -y --no-install-recommends \\\n")
	for _, row := range aptSharedRows {
		fmt.Fprintf(&b, "        %s \\\n", row.pkgs)
	}
	for _, r := range runtimes {
		if len(r.AptPkgs) > 0 {
			fmt.Fprintf(&b, "        %s \\\n", strings.Join(r.AptPkgs, " "))
		}
	}
	b.WriteString("    && rm -rf /var/lib/apt/lists/*\n")
	return b.String()
}

// miseInstallRegion generates the from-source compile RUN: the runtimes' build
// headers install, the mise install runs against /etc/mise, then the headers
// purge. $buildDeps stays unquoted so it word-splits into separate apt args.
func miseInstallRegion(runtimes []runtime.Runtime) string {
	var buildDeps []string
	for _, r := range runtimes {
		buildDeps = append(buildDeps, r.AptPkgsTemp...)
	}
	return fmt.Sprintf(`# Ruby build headers: install, compile Ruby, then purge (runtime libs kept above)
# $buildDeps intentionally unquoted below so it word-splits into separate apt args
# HOME is pinned to /root so the install's npm junk (playwright) lands in
# the cleaned home, never the ccbox home the mounts share.
# hadolint ignore=SC2086
RUN set -eux; \
    export HOME=/root; \
    apt-get update; \
    buildDeps='%s'; \
    apt-get install -y --no-install-recommends $buildDeps; \
    MISE_DATA_DIR=/usr/local/share/mise mise install; \
    apt-get purge -y --auto-remove $buildDeps; \
    rm -rf /var/lib/apt/lists/*
`, strings.Join(buildDeps, " "))
}

// runtimeEnvRegion generates one ENV per runtime env var, keys sorted — Go map
// iteration is randomized, and the image doesn't care about the order.
func runtimeEnvRegion(runtimes []runtime.Runtime) string {
	env := make(map[string]string)
	for _, r := range runtimes {
		maps.Copy(env, r.EnvVars)
	}
	var b strings.Builder
	for _, k := range slices.Sorted(maps.Keys(env)) {
		fmt.Fprintf(&b, "ENV %s=%s\n", k, env[k])
	}
	return b.String()
}

// pathSharedEntries lead the container PATH: the clis volume's bin (ccboxtools
// installs the coding CLI there at start), mise's shims, the user's private bin.
var pathSharedEntries = []string{
	"/opt/ccbox/clis/bin",
	"/home/ccbox/.local/share/mise/shims",
	"/home/ccbox/.local/bin",
}

// pathRegion generates the user PATH: the shared entries leading, then the
// runtimes' bin entries, then the inherited PATH.
func pathRegion(runtimes []runtime.Runtime) string {
	entries := slices.Clone(pathSharedEntries)
	for _, r := range runtimes {
		entries = append(entries, r.BinEntries...)
	}
	entries = append(entries, "$PATH")
	return "ENV PATH=" + strings.Join(entries, ":") + "\n"
}

// preCreateSharedDirs join the runtimes' cache volumes: the generic volume
// mountpoints, plus the clis root and opencode's config dir's parent.
var preCreateSharedDirs = []string{
	"/home/ccbox/.cache",
	"/home/ccbox/.local",
	"/home/ccbox/.config",
	"/opt/ccbox/clis",
}

// preCreateRegion generates the mountpoint mkdir RUN — named volumes inherit
// uid 1000 only off pre-created dirs — folding dirs at lineWidth, one apart,
// under `RUN mkdir -p `.
const (
	lineWidth   = 100
	mkdirCmd    = "RUN mkdir -p"
	mkdirIndent = "             "
)

func preCreateRegion(runtimes []runtime.Runtime) string {
	var dirs []string
	for _, r := range runtimes {
		dirs = append(dirs, r.CacheVolumes...)
	}
	dirs = append(dirs, preCreateSharedDirs...)

	var b strings.Builder
	b.WriteString(mkdirCmd)
	width := len(mkdirCmd)
	for _, dir := range dirs {
		if width+1+len(dir) > lineWidth {
			b.WriteString(" \\\n" + mkdirIndent)
			width = len(mkdirIndent)
		} else {
			b.WriteString(" ")
			width++
		}
		b.WriteString(dir)
		width += len(dir)
	}
	b.WriteString(` && \
    git config --file /home/ccbox/.gitconfig --add safe.directory '*'
`)
	return b.String()
}
