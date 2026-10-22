package prompt

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/kit/pick"
	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/util/slicex"
)

func TestMiseTools(t *testing.T) {
	t.Run("picked runtimes resolve to their tools and libs, node always in", func(t *testing.T) {
		stubRunTools(t, []pick.Item[toolModel]{pickedRuntime("python", "pypi:setuptools", "pypi:wheel")}, toolItems(mise.DevTools()), nil, nil)

		tools, packages, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.NoError(t, err)
		assert.Equal(t, []mise.Tool{
			{Name: "node"},
			{
				Name:               "python",
				PostInstallCmdTmpl: `python -m pip install --no-cache-dir{{range .}} {{.Name}}=={{.Version}}{{end}}`,
				PostInstallTools:   []mise.Tool{{Name: "pypi:setuptools"}, {Name: "pypi:wheel"}}, // the checked packages answer on the tool
			},
			{Name: "ripgrep"},
			{Name: "jq"},
			{Name: "yq"},
			{Name: "shellcheck"},
		}, tools)
		assert.Equal(t, []string{"libatomic1"}, packages)
	})

	t.Run("every package dropped answers empty", func(t *testing.T) {
		stubRunTools(t, []pick.Item[toolModel]{pickedRuntime("python")}, toolItems(mise.DevTools()), nil, nil)

		tools, _, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.NoError(t, err)
		assert.Equal(t, []mise.Tool{
			{Name: "node"},
			{
				Name:               "python",
				PostInstallCmdTmpl: `python -m pip install --no-cache-dir{{range .}} {{.Name}}=={{.Version}}{{end}}`,
				PostInstallTools:   []mise.Tool{}, // empty, not nil — resolve reads it as every package dropped
			},
			{Name: "ripgrep"},
			{Name: "jq"},
			{Name: "yq"},
			{Name: "shellcheck"},
		}, tools)
	})

	t.Run("a checked child joins its runtime, an unchecked one drops", func(t *testing.T) {
		stubRunTools(t, []pick.Item[toolModel]{pickedRuntime("go", "golangci-lint")}, toolItems(mise.DevTools()), nil, nil)

		tools, _, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.NoError(t, err)
		assert.Equal(t, []mise.Tool{
			{Name: "node"},
			{Name: "go"},
			{Name: "golangci-lint"},
			{Name: "ripgrep"},
			{Name: "jq"},
			{Name: "yq"},
			{Name: "shellcheck"},
		}, tools)

		stubRunTools(t, []pick.Item[toolModel]{pickedRuntime("go")}, toolItems(mise.DevTools()), nil, nil)

		tools, _, err = MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.NoError(t, err)
		assert.Equal(t, []mise.Tool{
			{Name: "node"},
			{Name: "go"},
			{Name: "ripgrep"},
			{Name: "jq"},
			{Name: "yq"},
			{Name: "shellcheck"},
		}, tools)
	})

	t.Run("the picked runtimes' apt libs are in the answer, sorted", func(t *testing.T) {
		stubRunTools(t, []pick.Item[toolModel]{pickedRuntime("ruby")}, toolItems(mise.DevTools()), nil, nil)

		_, packages, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.NoError(t, err)
		assert.Equal(t, []string{
			"libatomic1", "libffi8", "libgmp10", "libreadline8t64", "libssl3t64", "libyaml-0-2", "libzstd1", "zlib1g",
		}, packages)
	})

	t.Run("the independent tools' picks follow the runtimes'", func(t *testing.T) {
		stubRunTools(t, []pick.Item[toolModel]{pickedRuntime("go")},
			toolItems(mise.DevTools()), []pick.Item[toolModel]{pickedIndependent("hadolint"), pickedIndependent("npm:playwright")}, nil)

		tools, _, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.NoError(t, err)
		assert.Equal(t, []mise.Tool{
			{Name: "node"},
			{Name: "go"},
			{Name: "ripgrep"},
			{Name: "jq"},
			{Name: "yq"},
			{Name: "shellcheck"},
			{Name: "hadolint"},
			{
				Name:               "npm:playwright",
				PostInstallCmdTmpl: "playwright install-deps chromium && playwright install chromium",
				Comment:            "# --vnc flag uses this chromium as a browser",
				Hint:               "with chrome",
			},
		}, tools)
	})

	t.Run("the dev tools' picks follow the runtimes', ahead of the independent tools'", func(t *testing.T) {
		stubRunTools(t, []pick.Item[toolModel]{pickedRuntime("go")},
			[]pick.Item[toolModel]{pickedDev("ripgrep"), pickedDev("yq")},
			[]pick.Item[toolModel]{pickedIndependent("hadolint")}, nil)

		tools, _, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.NoError(t, err)
		assert.Equal(t, []mise.Tool{
			{Name: "node"},
			{Name: "go"},
			{Name: "ripgrep"},
			{Name: "yq"},
			{Name: "hadolint"},
		}, tools)
	})

	t.Run("an empty pick still includes node", func(t *testing.T) {
		stubRunTools(t, []pick.Item[toolModel]{}, nil, nil, nil)

		tools, packages, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.NoError(t, err)
		assert.Equal(t, []mise.Tool{{Name: "node"}}, tools)
		assert.Equal(t, []string{"libatomic1"}, packages)
	})

	t.Run("no picker answer errors", func(t *testing.T) {
		stubRunTools(t, nil, nil, nil, nil)

		_, _, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no tools selected")
	})

	t.Run("quitting the picker aborts", func(t *testing.T) {
		stubRunTools(t, nil, nil, nil, pick.ErrCanceled)

		_, _, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.ErrorIs(t, err, pick.ErrCanceled)
	})
}

func TestRuntimeItems(t *testing.T) {
	items := runtimeItems(mise.ToolState{})

	names := make([]string, 0, len(items))
	var goItem, python pick.Item[toolModel]
	for _, item := range items {
		names = append(names, item.Model.Name())
		switch item.Model.Name() {
		case "go":
			goItem = item
		case "python":
			python = item
		}
	}
	assert.Equal(t, []string{"go", "python", "ruby", "rust"}, names, "every selectable runtime in All()'s order — node always present, never an item")
	assert.Equal(t, []string{"golangci-lint"}, childNames(goItem), "the optionals listed as children")
	assert.Equal(t, []string{"pypi:setuptools", "pypi:wheel"}, childNames(python),
		"the postinstall packages as pickable children — an unchecked one dropping from the postinstall")

	t.Run("the written postinstall pins the packages — the inheritance hinted instead", func(t *testing.T) {
		state := mise.ToolState{PostInstalls: map[string]string{"python": "python -m pip install setuptools==70.0.0"}}

		python := runtimeItem(t, state, "python")

		assert.Equal(t, "inherit existing postinstall", python.Model.Tool.Hint)
		assert.Empty(t, python.Children, "the packages never re-fetch, never re-render")
	})

	t.Run("a plain pin keeps the children — the packages fetch at resolve time", func(t *testing.T) {
		state := mise.ToolState{Pins: map[string]string{"python": "3.13"}}

		python := runtimeItem(t, state, "python")

		assert.Equal(t, []string{"pypi:setuptools", "pypi:wheel"}, childNames(python))
	})
}

// runtimeItem is the named runtime's item as the state's picker shows it
func runtimeItem(t *testing.T, state mise.ToolState, name string) pick.Item[toolModel] {
	t.Helper()
	for _, item := range runtimeItems(state) {
		if item.Model.Name() == name {
			return item
		}
	}
	panic("no such runtime item: " + name)
}

func TestToolItems(t *testing.T) {
	t.Run("the dev tools' items follow DevTools' render order", func(t *testing.T) {
		assert.Equal(t, []string{"ripgrep", "jq", "yq", "shellcheck"}, itemNames(toolItems(mise.DevTools())))
	})

	t.Run("the independent tools' items follow IndependentTools' render order", func(t *testing.T) {
		items := toolItems(mise.IndependentTools())

		names := itemNames(items)
		assert.Equal(t, []string{"bats", "hadolint", "npm:playwright"}, names)

		playwright := items[slices.Index(names, "npm:playwright")]
		assert.Equal(t, "with chrome", playwright.Model.Hint(), "the tool's hint carries through the model")
		assert.Empty(t, playwright.Children, "a static postinstall has no packages to pick")
	})
}

// stubRunTools pins runFn's answers — the runtimes' checklist's, the dev
// tools', then the independent tools'
func stubRunTools(t *testing.T, rts, devTools, independentTools []pick.Item[toolModel], err error) {
	t.Helper()
	runFn = func(_ pick.Runner, ptrs ...any) error {
		require.Len(t, ptrs, 3)
		rtsPtr, ok := ptrs[0].(*[]pick.Item[toolModel])
		require.True(t, ok)
		devPtr, ok := ptrs[1].(*[]pick.Item[toolModel])
		require.True(t, ok)
		toolsPtr, ok := ptrs[2].(*[]pick.Item[toolModel])
		require.True(t, ok)
		*rtsPtr, *devPtr, *toolsPtr = rts, devTools, independentTools
		return err
	}
	t.Cleanup(func() { runFn = pick.Runner.Run })
}

// pickedRuntime is the named runtime item as respond answers it: the item
// with only its listed children checked
func pickedRuntime(name string, children ...string) pick.Item[toolModel] {
	for _, item := range runtimeItems(mise.ToolState{}) {
		if item.Model.Name() != name {
			continue
		}
		item.Children = slicex.Filter(item.Children, func(m toolModel) bool { return slices.Contains(children, m.Name()) })
		return item
	}
	panic("no such runtime item: " + name)
}

// pickedDev is the named dev tool item as respond answers it
func pickedDev(name string) pick.Item[toolModel] {
	for _, item := range toolItems(mise.DevTools()) {
		if item.Model.Name() == name {
			return item
		}
	}
	panic("no such dev item: " + name)
}

// pickedIndependent is the named independent tool item as respond answers it
func pickedIndependent(name string) pick.Item[toolModel] {
	for _, item := range toolItems(mise.IndependentTools()) {
		if item.Model.Name() == name {
			return item
		}
	}
	panic("no such independent item: " + name)
}

func itemNames(items []pick.Item[toolModel]) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Model.Name())
	}
	return names
}

func childNames(item pick.Item[toolModel]) []string {
	names := make([]string, 0, len(item.Children))
	for _, child := range item.Children {
		names = append(names, child.Name())
	}
	return names
}
