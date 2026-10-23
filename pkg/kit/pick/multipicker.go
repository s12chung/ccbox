package pick

import (
	"slices"
)

// Model is the payload an Item carries: its Name heads the row, its Hint the
// gray note parenthesized after it
type Model interface {
	Name() string
	Hint() string
}

// NewMultiPicker returns the checklist Picker for choosing several items, the
// named items pre-checked. No items panics.
func NewMultiPicker[M Model](items []Item[M], checked []string) Picker {
	if len(items) == 0 {
		panic("pick: no items to pick from")
	}
	p := &multiPicker[M]{checklist: slices.Clone(items)}
	checks := make(map[string]*bool, len(p.checklist))
	for i := range p.checklist {
		item := &p.checklist[i]
		item.checks = slices.Repeat([]bool{!item.RevealUnchecked}, len(item.Children))
		checks[item.Model.Name()] = &item.checked
		for j, child := range item.Children {
			checks[child.Name()] = &item.checks[j]
		}
		if item.IsDisabled {
			item.checked = true // a locked item is always picked
		}
	}
	for _, name := range checked {
		if check, ok := checks[name]; ok {
			*check = true
		}
	}
	return p
}

// Item is one checklist entry: children list the models revealed beneath it
// while it's checked — each child unchecks alone. The checks live on the item.
type Item[M Model] struct {
	Model           M
	Children        []M
	IsDisabled      bool // locks the row on checked — rendered gray, enter never toggles it
	RevealUnchecked bool // starts the revealed children unchecked

	checked bool
	checks  []bool // the children's checks, one per child
}

// row flattens the item into its visible rows: itself, then its revealed
// children while it's checked
func (item *Item[M]) row() []row {
	rows := []row{{name: item.Model.Name(), hint: item.Model.Hint(), checked: &item.checked, disabled: item.IsDisabled}}
	if !item.checked {
		return rows
	}
	for j, child := range item.Children {
		rows = append(rows, row{name: child.Name(), hint: child.Hint(), checked: &item.checks[j], child: true})
	}
	return rows
}

// Flatten lists the items' models in row order: each item's model, then its
// children's — the checklist's answer, read back as models
func Flatten[M Model](items []Item[M]) []M {
	return FlattenFunc(items, func(model M, children []M) []M {
		return append([]M{model}, children...)
	})
}

// FlattenFunc lists the items as f answers them: each item's model and its
// checked children in f's shape
func FlattenFunc[M Model, A any](items []Item[M], f func(M, []M) []A) []A {
	out := make([]A, 0, len(items))
	for _, item := range items {
		out = append(out, f(item.Model, item.Children)...)
	}
	return out
}

// row is one visible checklist row: the option itself, or one of its revealed
// children — checked points at the check it renders and toggles
type row struct {
	name     string
	hint     string
	checked  *bool
	disabled bool
	child    bool
}

func (r row) toggle() {
	if r.disabled {
		return
	}
	*r.checked = !*r.checked
}

// view renders the row: the cursor, the check box and the name, children
// indented under their option
func (r row) view(onCursor bool) string {
	cursor := "  "
	if onCursor {
		cursor = strongStyle.Render("❯ ")
	}
	indent := ""
	if r.child {
		indent = lightStyle.Render(" ∟")
	}
	if r.disabled {
		return cursor + indent + lightStyle.Render("[x] "+r.name) + hintSuffix(r.hint)
	}
	box := defaultStyle.Render("[ ]")
	if *r.checked {
		box = strongStyle.Render("[x]")
	}
	return cursor + indent + box + " " + r.name + hintSuffix(r.hint)
}

// multiPicker is the checklist parts: the checklist — each item a row, its
// children rows revealed while it's checked
type multiPicker[M Model] struct {
	checklist []Item[M]
}

// rows counts the visible rows: the options plus their revealed children
func (p *multiPicker[M]) rows() int { return len(p.flatten()) }

// render draws the visible rows
func (p *multiPicker[M]) render(cursor int, focused bool) []string {
	flat := p.flatten()
	rows := make([]string, len(flat))
	for i, r := range flat {
		rows[i] = r.view(focused && i == cursor)
	}
	return rows
}

// picked reports any item is checked — children don't count on their own: a
// checked child implies its item
func (p *multiPicker[M]) picked() bool {
	return slices.ContainsFunc(p.checklist, func(item Item[M]) bool { return item.checked })
}

// enter toggles the cursor's row's check — an option's, or one revealed child's
func (p *multiPicker[M]) enter(cursor int) { p.flatten()[cursor].toggle() }

// respond writes the checked items into the caller's *[]Item[M] — each item
// its model with its checked children's models alone, an unchecked item
// dropping its children with it. Non-nil on an empty check set, so callers
// can tell it from the nil non-terminal read
func (p *multiPicker[M]) respond(ptr any) error {
	picked := make([]Item[M], 0, len(p.checklist))
	for i := range p.checklist {
		item := &p.checklist[i]
		if !item.checked {
			continue
		}
		var children []M
		for j, child := range item.Children {
			if item.checks[j] {
				children = append(children, child)
			}
		}
		picked = append(picked, Item[M]{Model: item.Model, Children: children})
	}
	return fill(ptr, picked)
}

// flatten lists the checklist's visible rows: each option's rows in order
func (p *multiPicker[M]) flatten() []row {
	var rows []row
	for i := range p.checklist {
		rows = append(rows, p.checklist[i].row()...)
	}
	return rows
}
