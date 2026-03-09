package tui

import "testing"

func TestComputeLayout_NormalTerminal_ProjectsNotFocused(t *testing.T) {
	l := ComputeLayout(80, 24, PanelMergeRequests)

	// usableHeight = 24 - 1 (status) - 1 (keybind) = 22
	usableHeight := 22

	if l.Width != 80 {
		t.Errorf("Width = %d, want 80", l.Width)
	}
	if l.Height != 24 {
		t.Errorf("Height = %d, want 24", l.Height)
	}

	// Projects panel collapsed to 3 lines
	if l.PanelHeights[PanelProjects] != 3 {
		t.Errorf("Projects panel height = %d, want 3 (collapsed)", l.PanelHeights[PanelProjects])
	}

	// Remaining 19 lines split among 3 panels: 19/3 = 6 remainder 1
	remaining := usableHeight - 3
	totalOtherPanels := 0
	for i := 1; i < 4; i++ {
		totalOtherPanels += l.PanelHeights[i]
	}
	if totalOtherPanels != remaining {
		t.Errorf("sum of other 3 panels = %d, want %d", totalOtherPanels, remaining)
	}

	// All panel heights should sum to usableHeight
	totalHeight := 0
	for _, h := range l.PanelHeights {
		totalHeight += h
	}
	if totalHeight != usableHeight {
		t.Errorf("total panel height = %d, want %d", totalHeight, usableHeight)
	}
}

func TestComputeLayout_NormalTerminal_ProjectsFocused(t *testing.T) {
	l := ComputeLayout(80, 24, PanelProjects)

	// usableHeight = 24 - 2 = 22, all 4 panels equal: 22/4 = 5 remainder 2
	usableHeight := 22

	// All 4 panels should share space equally (with remainder distributed)
	totalHeight := 0
	for _, h := range l.PanelHeights {
		totalHeight += h
	}
	if totalHeight != usableHeight {
		t.Errorf("total panel height = %d, want %d", totalHeight, usableHeight)
	}

	// Each panel should be 5 or 6 (22/4 = 5 rem 2)
	for i, h := range l.PanelHeights {
		if h < 5 || h > 6 {
			t.Errorf("panel %d height = %d, want 5 or 6", i, h)
		}
	}
}

func TestComputeLayout_SmallTerminal(t *testing.T) {
	l := ComputeLayout(80, 12, PanelPipelines)

	// usableHeight = 12 - 2 = 10, but clamped to min 12
	usableHeight := 12

	// Projects collapsed to 3
	if l.PanelHeights[PanelProjects] != 3 {
		t.Errorf("Projects panel height = %d, want 3", l.PanelHeights[PanelProjects])
	}

	totalHeight := 0
	for _, h := range l.PanelHeights {
		totalHeight += h
	}
	if totalHeight != usableHeight {
		t.Errorf("total panel height = %d, want %d", totalHeight, usableHeight)
	}

	if l.ContentHeight != usableHeight {
		t.Errorf("ContentHeight = %d, want %d", l.ContentHeight, usableHeight)
	}
}

func TestComputeLayout_LargeTerminal(t *testing.T) {
	l := ComputeLayout(200, 60, PanelIssues)

	// usableHeight = 60 - 2 = 58
	usableHeight := 58

	// Projects collapsed to 3
	if l.PanelHeights[PanelProjects] != 3 {
		t.Errorf("Projects panel height = %d, want 3", l.PanelHeights[PanelProjects])
	}

	// Remaining 55 lines split among 3 panels
	remaining := usableHeight - 3
	totalOtherPanels := 0
	for i := 1; i < 4; i++ {
		totalOtherPanels += l.PanelHeights[i]
	}
	if totalOtherPanels != remaining {
		t.Errorf("sum of other 3 panels = %d, want %d", totalOtherPanels, remaining)
	}

	totalHeight := 0
	for _, h := range l.PanelHeights {
		totalHeight += h
	}
	if totalHeight != usableHeight {
		t.Errorf("total panel height = %d, want %d", totalHeight, usableHeight)
	}
}

func TestComputeLayout_LargeTerminal_ProjectsFocused(t *testing.T) {
	l := ComputeLayout(200, 60, PanelProjects)

	// usableHeight = 58, all 4 equal: 58/4 = 14 rem 2
	usableHeight := 58

	totalHeight := 0
	for _, h := range l.PanelHeights {
		totalHeight += h
	}
	if totalHeight != usableHeight {
		t.Errorf("total panel height = %d, want %d", totalHeight, usableHeight)
	}

	for i, h := range l.PanelHeights {
		if h < 14 || h > 15 {
			t.Errorf("panel %d height = %d, want 14 or 15", i, h)
		}
	}
}

func TestComputeLayout_SidebarWidthMin(t *testing.T) {
	// Width 60: 60*45/100 = 27, clamped to min 35
	l := ComputeLayout(60, 24, PanelMergeRequests)

	if l.SidebarWidth != 35 {
		t.Errorf("SidebarWidth = %d, want 35 (min)", l.SidebarWidth)
	}
	if l.ContentWidth != 60-35 {
		t.Errorf("ContentWidth = %d, want %d", l.ContentWidth, 60-35)
	}
}

func TestComputeLayout_SidebarWidthMax(t *testing.T) {
	// Width 200: 200*45/100 = 90, clamped to max 75
	l := ComputeLayout(200, 24, PanelMergeRequests)

	if l.SidebarWidth != 75 {
		t.Errorf("SidebarWidth = %d, want 75 (max)", l.SidebarWidth)
	}
	if l.ContentWidth != 200-75 {
		t.Errorf("ContentWidth = %d, want %d", l.ContentWidth, 200-75)
	}
}

func TestComputeLayout_SidebarWidthNormal(t *testing.T) {
	// Width 80: 80*45/100 = 36, within bounds
	l := ComputeLayout(80, 24, PanelMergeRequests)

	if l.SidebarWidth != 36 {
		t.Errorf("SidebarWidth = %d, want 36", l.SidebarWidth)
	}
}

func TestComputeLayout_ContentWidthMinimum(t *testing.T) {
	// Very narrow terminal: width 40, sidebar 35, content would be 5 -> clamped to 10
	l := ComputeLayout(40, 24, PanelMergeRequests)

	if l.SidebarWidth != 35 {
		t.Errorf("SidebarWidth = %d, want 35", l.SidebarWidth)
	}
	if l.ContentWidth != 10 {
		t.Errorf("ContentWidth = %d, want 10 (min)", l.ContentWidth)
	}
}

func TestComputeLayout_ContentWidthIsWidthMinusSidebar(t *testing.T) {
	l := ComputeLayout(120, 40, PanelPipelines)

	expected := 120 - l.SidebarWidth
	if l.ContentWidth != expected {
		t.Errorf("ContentWidth = %d, want %d (width - sidebar)", l.ContentWidth, expected)
	}
}

func TestComputeLayout_StatusAndKeybindBarHeights(t *testing.T) {
	l := ComputeLayout(80, 24, PanelMergeRequests)

	if l.StatusBarHeight != 1 {
		t.Errorf("StatusBarHeight = %d, want 1", l.StatusBarHeight)
	}
	if l.KeybindBarHeight != 1 {
		t.Errorf("KeybindBarHeight = %d, want 1", l.KeybindBarHeight)
	}
}

func TestComputeLayout_ContentHeightEqualsUsableHeight(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		panel  PanelID
	}{
		{"normal", 80, 24, PanelMergeRequests},
		{"large", 200, 60, PanelPipelines},
		{"projects focused", 80, 24, PanelProjects},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := ComputeLayout(tt.width, tt.height, tt.panel)

			usable := tt.height - l.StatusBarHeight - l.KeybindBarHeight
			if usable < 12 {
				usable = 12
			}
			if l.ContentHeight != usable {
				t.Errorf("ContentHeight = %d, want %d", l.ContentHeight, usable)
			}
		})
	}
}
