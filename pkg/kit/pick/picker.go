package pick

// NewPicker returns the single-pick Picker for choosing one item. No items
// panics.
func NewPicker(items []string) Picker {
	if len(items) == 0 {
		panic("pick: no items to pick from")
	}
	return &picker{items: items, selected: -1}
}

// picker is the single-pick parts: the items, the one marked with the circle
type picker struct {
	items    []string
	selected int // -1 is nothing marked yet
}

func (p *picker) rows() int { return len(p.items) }

// render draws the items, the marked one circled
func (p *picker) render(cursor int, focused bool) []string {
	rows := make([]string, len(p.items))
	for i, item := range p.items {
		circle, style := "○", defaultStyle
		if i == p.selected {
			circle, style = "◉", strongStyle
		}
		mark := "  "
		if focused && i == cursor {
			mark = strongStyle.Render("❯ ")
		}
		rows[i] = mark + circle + " " + style.Render(item)
	}
	return rows
}

// enter marks the cursor's item
func (p *picker) enter(cursor int) { p.selected = cursor }

// picked reports an item is marked
func (p *picker) picked() bool { return p.selected >= 0 }

// respond writes the marked item into the caller's *string; nothing marked
// reads empty
func (p *picker) respond(ptr any) error {
	item := ""
	if p.selected >= 0 {
		item = p.items[p.selected]
	}
	return fill(ptr, item)
}
