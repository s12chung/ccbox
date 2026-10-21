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
	t.Run("picked runtimes resolve to their tools and libs, node riding", func(t *testing.T) {
		stubRunTools(t, []pick.Item[toolModel]{pickedRuntime("python")}, nil, nil)

		tools, packages, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.NoError(t, err)
		assert.Equal(t, []mise.Tool{
			{Name: "node"},
			{
				Name:               "python", // the packages ride the postinstall, never the picker
				PostInstallCmdTmpl: `python -m pip install --no-cache-dir{{range .}} {{.Name}}=={{.Version}}{{end}}`,
				PostInstallNames:   []string{"pypi:setuptools", "pypi:wheel"},
			},
			{Name: "ripgrep", Comment: "# Dev-workflow CLIs"},
			{Name: "jq"},
			{Name: "yq"},
			{Name: "shellcheck"},
		}, tools)
		assert.Equal(t, []string{"libatomic1"}, packages)
	})

	t.Run("a checked child rides its runtime, an unchecked one drops", func(t *testing.T) {
		stubRunTools(t, []pick.Item[toolModel]{pickedRuntime("go", "golangci-lint")}, nil, nil)

		tools, _, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.NoError(t, err)
		assert.Equal(t, []mise.Tool{
			{Name: "node"},
			{Name: "go"},
			{Name: "golangci-lint"},
			{Name: "ripgrep", Comment: "# Dev-workflow CLIs"},
			{Name: "jq"},
			{Name: "yq"},
			{Name: "shellcheck"},
		}, tools)

		stubRunTools(t, []pick.Item[toolModel]{pickedRuntime("go")}, nil, nil)

		tools, _, err = MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.NoError(t, err)
		assert.Equal(t, []mise.Tool{
			{Name: "node"},
			{Name: "go"},
			{Name: "ripgrep", Comment: "# Dev-workflow CLIs"},
			{Name: "jq"},
			{Name: "yq"},
			{Name: "shellcheck"},
		}, tools)
	})

	t.Run("the picked runtimes' apt libs ride the answer, sorted", func(t *testing.T) {
		stubRunTools(t, []pick.Item[toolModel]{pickedRuntime("ruby")}, nil, nil)

		_, packages, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.NoError(t, err)
		assert.Equal(t, []string{
			"libatomic1", "libffi8", "libgmp10", "libreadline8t64", "libssl3t64", "libyaml-0-2", "libzstd1", "zlib1g",
		}, packages)
	})

	t.Run("the independent tools' picks ride the runtimes'", func(t *testing.T) {
		stubRunTools(t, []pick.Item[toolModel]{pickedRuntime("go")},
			[]pick.Item[toolModel]{pickedIndependent("hadolint"), pickedIndependent("npm:playwright")}, nil)

		tools, _, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.NoError(t, err)
		assert.Equal(t, []mise.Tool{
			{Name: "node"},
			{Name: "go"},
			{Name: "hadolint"},
			{
				Name:               "npm:playwright",
				PostInstallCmdTmpl: "playwright install-deps chromium && playwright install chromium",
				Comment:            "# --vnc flag uses this chromium as a browser",
				Hint:               "with chrome",
			},
			{Name: "ripgrep", Comment: "# Dev-workflow CLIs"},
			{Name: "jq"},
			{Name: "yq"},
			{Name: "shellcheck"},
		}, tools)
	})

	t.Run("an empty pick still rides node", func(t *testing.T) {
		stubRunTools(t, []pick.Item[toolModel]{}, nil, nil)

		tools, packages, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.NoError(t, err)
		assert.Equal(t, []mise.Tool{
			{Name: "node"},
			{Name: "ripgrep", Comment: "# Dev-workflow CLIs"},
			{Name: "jq"},
			{Name: "yq"},
			{Name: "shellcheck"},
		}, tools)
		assert.Equal(t, []string{"libatomic1"}, packages)
	})

	t.Run("no picker answer errors", func(t *testing.T) {
		stubRunTools(t, nil, nil, nil)

		_, _, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no tools selected")
	})

	t.Run("quitting the picker aborts", func(t *testing.T) {
		stubRunTools(t, nil, nil, pick.ErrCanceled)

		_, _, err := MiseTools("~/.ccbox/mise.toml", mise.ToolState{})
		require.ErrorIs(t, err, pick.ErrCanceled)
	})
}

func TestRuntimeItems(t *testing.T) {
	items := runtimeItems()

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
	assert.Equal(t, []string{"go", "python", "ruby"}, names, "every selectable runtime in All()'s order — node rides, never an item")
	assert.Equal(t, []string{"golangci-lint"}, childNames(goItem), "the optionals ride as children")
	assert.Empty(t, childNames(python), "python's packages ride its postinstall, never the picker")
}

func TestIndependentItems(t *testing.T) {
	items := independentItems()

	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Model.Name())
	}
	assert.Equal(t, []string{"bats", "hadolint", "npm:playwright"}, names)

	playwright := items[slices.Index(names, "npm:playwright")]
	assert.Equal(t, "with chrome", playwright.Model.Hint(), "the tool's hint rides the model")
}

// stubRunTools pins runFn's two answers — the runtimes' checklist's, then the
// independent tools'
func stubRunTools(t *testing.T, rts, independentTools []pick.Item[toolModel], err error) {
	t.Helper()
	runFn = func(_ pick.Runner, ptrs ...any) error {
		require.Len(t, ptrs, 2)
		rtsPtr, ok := ptrs[0].(*[]pick.Item[toolModel])
		require.True(t, ok)
		toolsPtr, ok := ptrs[1].(*[]pick.Item[toolModel])
		require.True(t, ok)
		*rtsPtr, *toolsPtr = rts, independentTools
		return err
	}
	t.Cleanup(func() { runFn = pick.Runner.Run })
}

// pickedRuntime is the named runtime item as respond answers it: the item
// with its listed children alone riding
func pickedRuntime(name string, children ...string) pick.Item[toolModel] {
	for _, item := range runtimeItems() {
		if item.Model.Name() != name {
			continue
		}
		item.Children = slicex.Filter(item.Children, func(m toolModel) bool { return slices.Contains(children, m.Name()) })
		return item
	}
	panic("no such runtime item: " + name)
}

// pickedIndependent is the named independent tool item as respond answers it
func pickedIndependent(name string) pick.Item[toolModel] {
	for _, item := range independentItems() {
		if item.Model.Name() == name {
			return item
		}
	}
	panic("no such independent item: " + name)
}

func childNames(item pick.Item[toolModel]) []string {
	names := make([]string, 0, len(item.Children))
	for _, child := range item.Children {
		names = append(names, child.Name())
	}
	return names
}
