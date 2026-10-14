package runtime

import (
	"slices"

	"github.com/s12chung/ccbox/pkg/mise"
)

// FromMiseToolNames resolves a mise config's [tools] names to the runtimes they
// select — node rides every resolution; non-runtime tools (the dev CLIs, the
// image's independent tools) drop
func FromMiseToolNames(toolNames []string) MiseRuntimes {
	names := append([]string{Node().Name}, toolNames...) // node rides every resolution
	var runtimes []Runtime
	for _, rt := range All() {
		if slices.Contains(names, rt.Name) {
			runtimes = append(runtimes, rt)
		}
	}
	return MiseRuntimes{Runtimes: runtimes}
}

// MiseRuntimes is the runtimes a mise config's [tools] table selects: the one
// input the Dockerfile regions and the run's cache volumes compose from.
type MiseRuntimes struct {
	// Runtimes is the selection plus node, in All()'s order
	Runtimes []Runtime
}

// Tools flattens the selection's tools — the mise config's [tools] region's
// input
func (m MiseRuntimes) Tools() []mise.Tool {
	var tools []mise.Tool
	for _, rt := range m.Runtimes {
		tools = append(tools, rt.MiseTool)
		tools = append(tools, rt.MiseToolsOptional...)
	}
	return tools
}

// CacheVolumes concatenates the selection's cache volumes
func (m MiseRuntimes) CacheVolumes() []string {
	var volumes []string
	for _, rt := range m.Runtimes {
		volumes = append(volumes, rt.CacheVolumes...)
	}
	return volumes
}
