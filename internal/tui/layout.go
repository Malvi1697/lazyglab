package tui

// Layout holds computed dimensions for the panel layout.
type Layout struct {
	// Total terminal dimensions
	Width  int
	Height int

	// Left sidebar
	SidebarWidth int

	// Individual panel heights (for the 4 sidebar panels)
	PanelHeights [4]int

	// Main content area
	ContentWidth  int
	ContentHeight int

	// Status bar + keybind hint bar
	StatusBarHeight  int
	KeybindBarHeight int
}

// ComputeLayout calculates panel dimensions based on terminal size.
func ComputeLayout(width, height int) Layout {
	l := Layout{
		Width:            width,
		Height:           height,
		StatusBarHeight:  1,
		KeybindBarHeight: 1,
	}

	// Sidebar takes ~30% of width, min 25, max 50
	l.SidebarWidth = width * 30 / 100
	if l.SidebarWidth < 25 {
		l.SidebarWidth = 25
	}
	if l.SidebarWidth > 50 {
		l.SidebarWidth = 50
	}

	// Content area is the rest
	// Account for borders (2 chars each side)
	l.ContentWidth = width - l.SidebarWidth - 2
	if l.ContentWidth < 10 {
		l.ContentWidth = 10
	}

	// Usable height for panels (minus status bar, keybind bar, and borders)
	usableHeight := height - l.StatusBarHeight - l.KeybindBarHeight - 2

	// Distribute height evenly across 4 panels, accounting for borders
	panelHeight := usableHeight / 4
	remainder := usableHeight - (panelHeight * 4)
	for i := range l.PanelHeights {
		l.PanelHeights[i] = panelHeight
		// Distribute remainder to first panels
		if i < remainder {
			l.PanelHeights[i]++
		}
	}

	l.ContentHeight = usableHeight

	return l
}
