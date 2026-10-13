package pick

import (
	"slices"
)

// NewMultiPicker returns the checklist Picker for choosing several items, the
// named items pre-checked. No items panics.
func NewMultiPicker(items []Item, checked []string) Picker {
	if len(items) == 0 {
		panic("pick: no items to pick from")
	}
	p := &multiPicker{checklist: slices.Clone(items)}
	byName := make(map[string]*Item, len(p.checklist))
	for i := range p.checklist {
		item := &p.checklist[i]
		item.checks = slices.Repeat([]bool{true}, len(item.Children)) // a reveal starts checked
		byName[item.Name] = item
	}
	for _, name := range checked {
		if item, ok := byName[name]; ok {
			item.checked = true
		}
	}
	return p
}

// Item is one checklist entry: children list the names revealed checked
// beneath it while it's checked — each child unchecks alone. The checks ride
// the item.
type Item struct {
	Name     string
	Children []string

	// Hint is the gray note parenthesized after the name
	Hint string

	checked bool
	checks  []bool // the children's checks, one per child
}

// row flattens the item into its visible rows: itself, then its revealed
// children while it's checked
func (item *Item) row() []row {
	rows := []row{{name: item.Name, hint: item.Hint, checked: &item.checked}}
	if !item.checked {
		return rows
	}
	for j, child := range item.Children {
		rows = append(rows, row{name: child, checked: &item.checks[j], child: true})
	}
	return rows
}

// row is one visible checklist row: the option itself, or one of its revealed
// children — checked points at the check it renders and toggles
type row struct {
	name    string
	hint    string
	checked *bool
	child   bool
}

func (r row) toggle() { *r.checked = !*r.checked }

// response is the row's pick: its name when checked
func (r row) response() (string, bool) { return r.name, *r.checked }

// view renders the row: the cursor, the check box and the name, children
// indented under their option
func (r row) view(onCursor bool) string {
	cursor := "  "
	if onCursor {
		cursor = strongStyle.Render("❯ ")
	}
	indent := ""
	if r.child {
		indent = "  "
	}
	box := defaultStyle.Render("[ ]")
	if *r.checked {
		box = strongStyle.Render("[x]")
	}
	hint := ""
	if r.hint != "" {
		hint = " " + lightStyle.Render("("+r.hint+")")
	}
	return cursor + indent + box + " " + r.name + hint
}

// multiPicker is the checklist parts: the checklist — each item a row, its
// children rows revealed while it's checked
type multiPicker struct {
	checklist []Item
}

// rows counts the visible rows: the options plus their revealed children
func (p *multiPicker) rows() int { return len(p.flatten()) }

// render draws the visible rows
func (p *multiPicker) render(cursor int, focused bool) []string {
	flat := p.flatten()
	rows := make([]string, len(flat))
	for i, r := range flat {
		rows[i] = r.view(focused && i == cursor)
	}
	return rows
}

// picked reports any item is checked — children don't count: they ride their
// item's check
func (p *multiPicker) picked() bool {
	return slices.ContainsFunc(p.checklist, func(item Item) bool { return item.checked })
}

// enter toggles the cursor's row's check — an option's, or one revealed child's
func (p *multiPicker) enter(cursor int) { p.flatten()[cursor].toggle() }

// respond writes the checked names into the caller's *[]string — non-nil on
// an empty check set, so callers can tell it from the nil non-terminal read
func (p *multiPicker) respond(ptr any) error {
	picked := make([]string, 0, len(p.checklist))
	for i := range p.checklist {
		for _, r := range p.checklist[i].row() {
			if name, ok := r.response(); ok {
				picked = append(picked, name)
			}
		}
	}
	return fill(ptr, picked)
}

// flatten lists the checklist's visible rows: each option's rows in order
func (p *multiPicker) flatten() []row {
	var rows []row
	for i := range p.checklist {
		rows = append(rows, p.checklist[i].row()...)
	}
	return rows
}
