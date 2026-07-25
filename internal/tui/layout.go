package tui

// Layout holds computed dimensions for the panel layout.
type Layout struct {
	// Total terminal dimensions
	Width  int
	Height int

	// Left sidebar
	SidebarWidth int

	// Individual panel heights (for the 4 sidebar panels), including borders
	PanelHeights [4]int

	// Main content area (total width including borders)
	ContentWidth  int
	ContentHeight int

	// Status bar + keybind hint bar
	StatusBarHeight  int
	KeybindBarHeight int
}

// ComputeLayout calculates panel dimensions based on terminal size and the set
// of visible panels (in display order). PanelHeights is keyed by PanelID; hidden
// panels get height 0.
func ComputeLayout(width, height int, activePanel PanelID, panels []PanelID) Layout {
	l := Layout{
		Width:            width,
		Height:           height,
		StatusBarHeight:  1,
		KeybindBarHeight: 1,
	}

	// Sidebar takes ~45% of width, min 35, max 75
	l.SidebarWidth = width * 45 / 100
	if l.SidebarWidth < 35 {
		l.SidebarWidth = 35
	}
	if l.SidebarWidth > 75 {
		l.SidebarWidth = 75
	}

	// Content area is the rest (total width including borders)
	l.ContentWidth = width - l.SidebarWidth
	if l.ContentWidth < 10 {
		l.ContentWidth = 10
	}

	// Usable height for panels (minus status bar and keybind bar)
	usableHeight := height - l.StatusBarHeight - l.KeybindBarHeight
	if usableHeight < 12 {
		usableHeight = 12
	}
	l.ContentHeight = usableHeight

	if len(panels) == 0 {
		return l
	}

	projectsVisible := false
	for _, p := range panels {
		if p == PanelProjects {
			projectsVisible = true
		}
	}

	// distribute spreads total across ids, giving the remainder to the first few.
	distribute := func(ids []PanelID, total int) {
		n := len(ids)
		if n == 0 {
			return
		}
		base := total / n
		rem := total - base*n
		for i, id := range ids {
			h := base
			if i < rem {
				h++
			}
			l.PanelHeights[id] = h
		}
	}

	if activePanel == PanelProjects || !projectsVisible {
		// All visible panels share space equally.
		distribute(panels, usableHeight)
		return l
	}

	// Projects collapsed to 3 lines; remaining height split among the others.
	const collapsed = 3
	l.PanelHeights[PanelProjects] = collapsed
	others := make([]PanelID, 0, len(panels))
	for _, p := range panels {
		if p != PanelProjects {
			others = append(others, p)
		}
	}
	distribute(others, usableHeight-collapsed)

	return l
}
