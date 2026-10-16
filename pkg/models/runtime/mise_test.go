package runtime

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/mise"
)

func TestSelectable(t *testing.T) {
	names := make([]string, 0, len(Selectable()))
	for _, rt := range Selectable() {
		names = append(names, rt.Name)
	}
	assert.Equal(t, SelectableNames(), names)
}

func TestSelectableNames(t *testing.T) {
	names := SelectableNames()

	assert.True(t, slices.IsSorted(names))
	assert.NotContains(t, names, Node().Name, "node rides every resolution, it's not selectable")
}

func TestFromNames(t *testing.T) {
	tests := []struct {
		name         string
		runtimeNames []string
		want         []string
	}{
		{"no tools is node alone", nil, []string{"node"}},
		{"the selection rides node, in All()'s order", []string{"ruby", "go"}, []string{"go", "node", "ruby"}},
		{"non-runtime tools drop", []string{"go", "hadolint", "npm:playwright", "bogus"}, []string{"go", "node"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names := make([]string, 0, len(tt.want))
			for _, rt := range FromNames(tt.runtimeNames).Runtimes {
				names = append(names, rt.Name)
			}
			assert.Equal(t, tt.want, names)
		})
	}
}

func TestMiseRuntimes_Tools(t *testing.T) {
	// every runtime's tools flatten in All()'s order, each optional tool riding its runtime
	var want []mise.Tool
	for _, rt := range All() {
		want = append(want, rt.MiseTool)
		want = append(want, rt.MiseToolsOptional...)
	}
	assert.Equal(t, want, MiseRuntimes{Runtimes: All()}.Tools())
}

func TestMiseRuntimes_Packages(t *testing.T) {
	assert.Equal(t, []string{
		"libatomic1", "libffi8", "libgmp10", "libreadline8t64", "libssl3t64", "libyaml-0-2", "libzstd1", "zlib1g",
	}, MiseRuntimes{Runtimes: All()}.Packages())

	// libs two runtimes share render once — a repeat would redeclare the TOML key
	overlapping := MiseRuntimes{Runtimes: []Runtime{
		{Name: "a", AptPkgs: []string{"libz", "liba"}},
		{Name: "b", AptPkgs: []string{"libz", "libm"}},
	}}
	assert.Equal(t, []string{"liba", "libm", "libz"}, overlapping.Packages())
}

func TestMiseRuntimes_CacheVolumes(t *testing.T) {
	assert.Equal(t, AllCacheVolumes(), MiseRuntimes{Runtimes: All()}.CacheVolumes())
	assert.Equal(t, Node().CacheVolumes, FromNames(nil).CacheVolumes(), "node's volumes ride every resolution")
}
