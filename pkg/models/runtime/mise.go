package runtime

import (
	"slices"

	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/util/slicex"
)

// FromMiseConfig is the picked mise config's runtime selection — the config's
// [tools] pins resolved to runtimes
func FromMiseConfig(projectDir string) (MiseRuntimes, error) {
	state, err := mise.ReadToolState(projectDir)
	if err != nil {
		return nil, err
	}
	pinned := make(map[string]bool, len(state.Pins)+1)
	for name := range state.Pins {
		pinned[name] = true
	}
	pinned[Node().Name] = true // node rides every resolution — the image's tooling runs on it
	var runtimes MiseRuntimes
	for _, rt := range All() {
		if pinned[rt.Name] {
			runtimes = append(runtimes, rt)
		}
	}
	return runtimes, nil
}

// MiseRuntimes is the runtimes a mise config's [tools] table selects: the one
// input the Dockerfile regions and the run's cache volumes compose from.
type MiseRuntimes []Runtime

// Packages flattens the selection's apt libs, repeats dropped and sorted — a
// repeat would redeclare a TOML key
func (m MiseRuntimes) Packages() []string {
	var pkgs []string
	for _, rt := range m {
		pkgs = append(pkgs, rt.AptPkgs...)
	}
	pkgs = slicex.Unique(pkgs)
	slices.Sort(pkgs)
	return pkgs
}

// CacheVolumes concatenates the selection's cache volumes
func (m MiseRuntimes) CacheVolumes() []string {
	var volumes []string
	for _, rt := range m {
		volumes = append(volumes, rt.CacheVolumes...)
	}
	return volumes
}
