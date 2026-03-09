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

// ComputeLayout calculates panel dimensions based on terminal size.
func ComputeLayout(width, height int, activePanel PanelID) Layout {
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

	// Equal distribution across 4 panels
	panelHeight := usableHeight / 4
	remainder := usableHeight - (panelHeight * 4)
	for i := range l.PanelHeights {
		l.PanelHeights[i] = panelHeight
		if i < remainder {
			l.PanelHeights[i]++
		}
	}

	l.ContentHeight = usableHeight

	return l
}
