package prompt

import (
	"fmt"
	"maps"
	"slices"

	"github.com/s12chung/ccbox/pkg/kit/pick"
	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/slicex"
)

// MiseTools asks which mise tools to install; the answer is the picked tools
// with the runtimes' apt libs — a zero state a fresh config, nothing written
// to prefill or ride
func MiseTools(savePath string, state mise.ToolState) ([]mise.Tool, []string, error) {
	var pickedRuntimes, pickedIndependent []pick.Item[toolModel]
	checked := slices.Sorted(maps.Keys(state.Pins))
	err := runFn(pick.NewRunner(pick.RunnerOptions{
		Header:      "Select the tools to install",
		Subtext:     []string{"(node is included)"},
		SubmitLabel: "Save to " + userdir.Tilde(savePath),
	},
		pick.NewMultiPicker(runtimeItems(state), checked),
		pick.NewMultiPicker(independentItems(), checked),
	), &pickedRuntimes, &pickedIndependent)
	if err != nil {
		return nil, nil, err
	}
	if pickedRuntimes == nil { // no terminal to show the picker
		return nil, nil, fmt.Errorf("no tools selected: rerun in a terminal to pick them, or edit the [tools] region in %s", savePath)
	}

	node := runtime.Node()
	runtimes := make(runtime.MiseRuntimes, 0, len(pickedRuntimes)+1)
	runtimes = append(runtimes, node) // node rides every pick
	for _, item := range pickedRuntimes {
		runtimes = append(runtimes, item.Model.rt)
	}
	tools := append([]mise.Tool{node.MiseTool},
		pick.FlattenFunc(pickedRuntimes, flattenNonPostInstall)...)
	tools = append(tools, pick.FlattenFunc(pickedIndependent, flattenNonPostInstall)...)
	return append(tools, mise.DevTools()...), runtimes.Packages(), nil
}

// toolModel is the mise.Tool as a pick.Model — the Tool's own Tool and Hint
// fields block the Name and Hint methods, so it wraps instead of implementing.
// A runtime item's model also carries rt, its runtime's registry value, so
// the answer maps to the config content without re-resolving the registry
// (children and independent tools carry none)
type toolModel struct {
	mise.Tool

	rt runtime.Runtime
}

func (m toolModel) Name() string { return m.Tool.Name }
func (m toolModel) Hint() string { return m.Tool.Hint }

// runtimeItems lists the picker's runtime items — every selectable runtime as
// its mise tool, its optionals and postinstall packages as children; a written
// postinstall hints its inheritance instead
func runtimeItems(state mise.ToolState) []pick.Item[toolModel] {
	all := runtime.All()
	items := make([]pick.Item[toolModel], 0, len(all)-1)
	for _, rt := range all {
		if rt.Name == runtime.Node().Name {
			continue
		}
		model := toolModel{Tool: rt.MiseTool, rt: rt}
		children := toolModels(rt.MiseToolsOptional)
		if state.PostInstalls[rt.Name] != "" {
			model.Tool.Hint = "inherit existing postinstall"
		} else {
			children = append(children, toolModels(rt.MiseTool.PostInstallTools)...)
		}
		items = append(items, pick.Item[toolModel]{Model: model, Children: children})
	}
	return items
}

// independentItems lists the picker's independent tool items
func independentItems() []pick.Item[toolModel] {
	return slicex.Map(mise.IndependentTools(), func(t mise.Tool) pick.Item[toolModel] {
		return pick.Item[toolModel]{Model: toolModel{Tool: t}}
	})
}

func toolModels(tools []mise.Tool) []toolModel {
	return slicex.Map(tools, func(t mise.Tool) toolModel { return toolModel{Tool: t} })
}

// modelTools unpacks the models to their tools
func modelTools(models []toolModel) []mise.Tool {
	return slicex.Map(models, func(m toolModel) mise.Tool { return m.Tool })
}

// flattenNonPostInstall flattens the mise tools, and keeps postinstall tools
// in the PostInstallTools field
func flattenNonPostInstall(model toolModel, children []toolModel) []mise.Tool {
	if len(model.PostInstallTools) == 0 {
		return append([]mise.Tool{model.Tool}, modelTools(children)...)
	}
	model.PostInstallTools = modelTools(children)
	return []mise.Tool{model.Tool}
}
