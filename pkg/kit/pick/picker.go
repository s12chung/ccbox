package pick

// NewPicker returns the single-pick Picker for choosing one item. No items
// panics.
func NewPicker[M Model](items []M) Picker {
	if len(items) == 0 {
		panic("pick: no items to pick from")
	}
	return &picker[M]{items: items, selected: -1}
}

// picker is the single-pick parts: the items, the one marked with the circle
type picker[M Model] struct {
	items    []M
	selected int // -1 is nothing marked yet
}

func (p *picker[M]) rows() int { return len(p.items) }

// render draws the items, the marked one circled
func (p *picker[M]) render(cursor int, focused bool) []string {
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
		rows[i] = mark + circle + " " + style.Render(item.Name()) + hintSuffix(item.Hint())
	}
	return rows
}

// enter marks the cursor's item
func (p *picker[M]) enter(cursor int) { p.selected = cursor }

// picked reports an item is marked
func (p *picker[M]) picked() bool { return p.selected >= 0 }

// respond writes the marked item into the caller's *M; nothing marked reads
// the zero model
func (p *picker[M]) respond(ptr any) error {
	var item M
	if p.selected >= 0 {
		item = p.items[p.selected]
	}
	return fill(ptr, item)
}
