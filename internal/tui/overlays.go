package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
	"github.com/Malvi1697/lazyglab/internal/util"
)

// overlayKind identifies which modal overlay (if any) is currently active.
type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayProject
	overlayBranch
	overlayHelp
	overlayConfirm
	overlayReconfig
	overlayFavorites
)

// confirmAction holds state for a pending confirmation dialog. The action runs
// only if the user confirms.
type confirmAction struct {
	prompt string
	action tea.Cmd
}

// confirm opens a confirmation dialog for a destructive action.
func (a *App) confirm(prompt string, action tea.Cmd) {
	a.pendingConfirm = &confirmAction{prompt: prompt, action: action}
	a.overlay = overlayConfirm
}

// mergeOverlay composites a centered overlay box onto the frame within the box's
// own columns, so the frame stays visible around it. Replacing whole lines (or
// only non-blank ones) either blanks out everything beside the box or lets the
// frame show through the box's own gaps.
func (a *App) mergeOverlay(frame, box string) string {
	boxLines := strings.Split(box, "\n")

	boxWidth := 0
	for _, l := range boxLines {
		if w := lipgloss.Width(l); w > boxWidth {
			boxWidth = w
		}
	}

	x := (a.width - boxWidth) / 2
	if x < 0 {
		x = 0
	}
	bgLines := strings.Split(frame, "\n")
	y := (len(bgLines) - len(boxLines)) / 2
	if y < 0 {
		y = 0
	}

	for i, boxLine := range boxLines {
		row := y + i
		if row < 0 || row >= len(bgLines) {
			continue
		}
		bg := bgLines[row]

		left := ansi.Truncate(bg, x, "")
		if w := lipgloss.Width(left); w < x {
			left += strings.Repeat(" ", x-w)
		}
		right := ansi.TruncateLeft(bg, x+lipgloss.Width(boxLine), "")

		// Reset between segments: a style left open by the frame must not colour
		// the box, and vice versa.
		bgLines[row] = left + "\x1b[0m" + boxLine + "\x1b[0m" + right
	}
	return strings.Join(bgLines, "\n")
}

// ============================================================================
// Confirm dialog
// ============================================================================

func (a *App) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", KeyEnter:
		var action tea.Cmd
		if a.pendingConfirm != nil {
			action = a.pendingConfirm.action
		}
		a.pendingConfirm = nil
		a.overlay = overlayNone
		return a, action
	default:
		a.pendingConfirm = nil
		a.overlay = overlayNone
		a.statusText = "Canceled"
		a.statusIsErr = false
		return a, nil
	}
}

func (a *App) renderConfirm() string {
	if a.pendingConfirm == nil {
		return ""
	}
	prompt := a.pendingConfirm.prompt
	hint := "y/Enter: confirm  n/Esc: cancel"

	innerWidth := len(prompt)
	if len(hint) > innerWidth {
		innerWidth = len(hint)
	}
	innerWidth += 4
	boxWidth := innerWidth + 4

	lines := []string{
		"",
		"  " + lipgloss.NewStyle().Bold(true).Foreground(components.ColorWarning).Render(prompt),
		"",
		"  " + components.HelpDescStyle.Render(hint),
		"",
	}
	return components.RenderBox("Confirm", lines, boxWidth, len(lines)+2, components.ColorWarning, components.ColorWarning)
}

// ============================================================================
// Branch picker
// ============================================================================

func (a *App) handleBranchPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if consumed, changed := a.branchFilter.handleKey(msg); consumed {
		if changed {
			a.branchCursor = 0
		}
		return a, nil
	}

	visible := a.visibleBranches()
	key := msg.String()
	switch {
	case key == KeyEscape && a.branchFilter.applied():
		a.branchFilter.reset()
		a.branchCursor = 0
		return a, nil
	case key == KeyEscape || key == KeyQuit:
		a.overlay = overlayNone
		a.branchFilter.reset()
		return a, nil
	case key == KeySearch:
		a.branchFilter.active = true
		if !a.branchFilter.applied() {
			a.branchCursor = 0
		}
		return a, nil
	case components.NavFor(key) != components.NavNone:
		_, boxHeight := a.overlayBoxSize()
		a.branchCursor = components.ApplyNav(components.NavFor(key), a.branchCursor, len(visible), boxHeight-4)
		return a, nil
	case key == KeyEnter:
		if a.branchCursor >= 0 && a.branchCursor < len(visible) {
			branch := visible[a.branchCursor]
			a.overlay = overlayNone
			a.branchFilter.reset()
			return a, func() tea.Msg { return views.BranchSelectedMsg{Branch: branch} }
		}
		return a, nil
	}
	return a, nil
}

// visibleBranches returns the branches matching the current search, in list
// order. The picker's cursor indexes this slice, not the full list.
func (a *App) visibleBranches() []gitlab.Branch {
	if !a.branchFilter.on() {
		return a.branches
	}
	out := make([]gitlab.Branch, 0, len(a.branches))
	for _, b := range a.branches {
		if a.branchFilter.matches(b.Name) {
			out = append(out, b)
		}
	}
	return out
}

func (a *App) renderBranchPicker() string {
	boxWidth, boxHeight := a.overlayBoxSize()
	maxVisible := boxHeight - 4
	if maxVisible < 3 {
		maxVisible = 3
	}

	visible := a.visibleBranches()

	var lines []string
	switch {
	case len(a.branches) == 0:
		lines = append(lines, "No branches found")
	case len(visible) == 0:
		lines = append(lines, components.HelpDescStyle.Render("No branch matches "+a.branchFilter.query))
	default:
		a.branchScroll = components.ScrollOffset(a.branchScroll, a.branchCursor, len(visible), maxVisible)
		for i := a.branchScroll; i < len(visible) && len(lines) < maxVisible; i++ {
			b := visible[i]
			marker := "  "
			if b.Default {
				marker = "* "
			} else if b.Protected {
				marker = "P "
			}
			activity := ""
			if !b.LastActivity.IsZero() {
				activity = components.HelpDescStyle.Render(" " + util.TimeAgo(b.LastActivity))
			}
			if i == a.branchCursor {
				line := components.SelectedItemStyle.Render(marker + b.Name)
				if activity != "" {
					line += activity
				}
				lines = append(lines, line)
			} else {
				lines = append(lines, fmt.Sprintf("%s%s%s", marker, b.Name, activity))
			}
		}
	}
	hint := "Enter: select  /: search  Esc: cancel"
	if a.branchFilter.active {
		hint = a.branchFilter.hint()
	}
	lines = append(lines, "", components.HelpDescStyle.Render(hint))

	title := fmt.Sprintf("Select Branch (%d)", len(a.branches))
	if a.branchFilter.on() {
		title = fmt.Sprintf("Select Branch (%d/%d)", len(visible), len(a.branches))
	}
	return components.RenderBox(title, lines, boxWidth, boxHeight, components.ColorPrimary, components.ColorPrimary)
}

// ============================================================================
// Project picker
// ============================================================================

func (a *App) handleProjectPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// While searching, typed characters extend the query rather than act as keys.
	if consumed, changed := a.projectFilter.handleKey(msg); consumed {
		if changed {
			a.projectCursor = 0
		}
		return a, nil
	}

	visible := a.visibleProjects()
	key := msg.String()
	switch {
	case key == KeyEscape && a.projectFilter.applied():
		// Drop the search first; the picker itself closes on the next Esc.
		a.projectFilter.reset()
		a.projectCursor = 0
		return a, nil
	case key == KeyEscape || key == KeyQuit:
		a.overlay = overlayNone
		a.projectFilter.reset()
		return a, nil
	case key == KeySearch:
		// Resume editing an applied query rather than starting over.
		a.projectFilter.active = true
		if !a.projectFilter.applied() {
			a.projectCursor = 0
		}
		return a, nil
	case components.NavFor(key) != components.NavNone:
		_, boxHeight := a.overlayBoxSize()
		a.projectCursor = components.ApplyNav(components.NavFor(key), a.projectCursor, len(visible), boxHeight-4)
		return a, nil
	case key == KeyFavorite:
		if a.projectCursor >= 0 && a.projectCursor < len(visible) {
			return a, a.toggleFavorite(visible[a.projectCursor].PathWithNamespace)
		}
		return a, nil
	case key == KeyEnter:
		if a.projectCursor >= 0 && a.projectCursor < len(visible) {
			proj := visible[a.projectCursor]
			a.overlay = overlayNone
			a.projectFilter.reset()
			return a, func() tea.Msg { return views.ProjectSelectedMsg{Project: proj} }
		}
		return a, nil
	}
	return a, nil
}

// visibleProjects returns the projects matching the current search, starred ones
// first and each group still ordered by recent activity. The picker's cursor
// indexes this slice, not the full list.
func (a *App) visibleProjects() []gitlab.Project {
	matching := a.projects
	if a.projectFilter.on() {
		matching = make([]gitlab.Project, 0, len(a.projects))
		for _, p := range a.projects {
			if a.projectFilter.matches(p.NameWithNamespace) || a.projectFilter.matches(p.PathWithNamespace) {
				matching = append(matching, p)
			}
		}
	}

	if len(a.favorites) == 0 {
		return matching
	}

	starred := make([]gitlab.Project, 0, len(a.favorites))
	rest := make([]gitlab.Project, 0, len(matching))
	for _, p := range matching {
		if a.isFavorite(p.PathWithNamespace) {
			starred = append(starred, p)
		} else {
			rest = append(rest, p)
		}
	}
	return append(starred, rest...)
}

// favoriteCount returns how many of the visible projects are starred. They are
// always the leading entries, so this doubles as the index of the first
// non-favorite — i.e. where the divider goes.
func (a *App) favoriteCount(visible []gitlab.Project) int {
	n := 0
	for _, p := range visible {
		if !a.isFavorite(p.PathWithNamespace) {
			break
		}
		n++
	}
	return n
}

func (a *App) renderProjectPicker() string {
	boxWidth, boxHeight := a.overlayBoxSize()
	maxVisible := boxHeight - 4
	if maxVisible < 3 {
		maxVisible = 3
	}
	innerWidth := boxWidth - 4

	visible := a.visibleProjects()

	var lines []string
	switch {
	case len(a.projects) == 0:
		lines = append(lines, "No projects found")
	case len(visible) == 0:
		lines = append(lines, components.HelpDescStyle.Render("No project matches "+a.projectFilter.query))
	default:
		lines = append(lines, a.projectRows(visible, innerWidth, maxVisible)...)
	}
	hint := "Enter: select  /: search  f: star  Esc: cancel"
	switch {
	case a.projectFilter.active:
		hint = a.projectFilter.hint() + components.HelpDescStyle.Render("   Enter: apply")
	case a.favoritesStatus != "":
		hint = components.Truncate(a.favoritesStatus, innerWidth)
	case a.projectFilter.applied():
		hint = a.projectFilter.hint() + components.HelpDescStyle.Render("   Enter: select  f: star  Esc: clear")
	}
	lines = append(lines, "", components.HelpDescStyle.Render(hint))

	title := fmt.Sprintf("Select Project (%d)", len(a.projects))
	if a.projectFilter.on() {
		title = fmt.Sprintf("Select Project (%d/%d)", len(visible), len(a.projects))
	}
	return components.RenderBox(title, lines, boxWidth, boxHeight, components.ColorPrimary, components.ColorPrimary)
}

// projectRows renders the visible slice of the project list, with a divider
// between the starred projects at the top and the rest. The divider occupies a
// line but is not selectable, so scrolling is computed over rendered rows while
// the cursor keeps indexing projects.
func (a *App) projectRows(visible []gitlab.Project, innerWidth, maxRows int) []string {
	divider := components.HelpSepStyle.Render(strings.Repeat("─", innerWidth))
	favCount := a.favoriteCount(visible)

	// Build every row first, remembering which row holds the cursor.
	var rows []string
	cursorRow := 0
	for i, p := range visible {
		if i == favCount && favCount > 0 {
			rows = append(rows, divider)
		}

		marker := "  "
		switch {
		case a.ctx != nil && a.ctx.Project != nil && a.ctx.Project.ID == p.ID:
			marker = "* "
		case a.isFavorite(p.PathWithNamespace):
			marker = "★ "
		}
		label := components.Truncate(marker+p.NameWithNamespace, innerWidth)
		if i == a.projectCursor {
			cursorRow = len(rows)
			label = components.SelectedItemStyle.Render(components.PadRight(label, innerWidth))
		}
		rows = append(rows, label)
	}

	// Scroll so the cursor's row keeps a margin of context around it.
	a.projectScroll = components.ScrollOffset(a.projectScroll, cursorRow, len(rows), maxRows)
	offset := a.projectScroll
	end := offset + maxRows
	if end > len(rows) {
		end = len(rows)
	}
	return rows[offset:end]
}

// overlayBoxSize returns a reasonable centered-box size for list overlays.
func (a *App) overlayBoxSize() (int, int) {
	boxWidth := a.width * 6 / 10
	if boxWidth > 70 {
		boxWidth = 70
	}
	if boxWidth < 30 {
		boxWidth = 30
	}
	if boxWidth > a.width-2 {
		boxWidth = a.width - 2
	}
	boxHeight := a.height * 7 / 10
	if boxHeight < 6 {
		boxHeight = 6
	}
	if boxHeight > a.height-2 {
		boxHeight = a.height - 2
	}
	return boxWidth, boxHeight
}

// ============================================================================
// Help overlay
// ============================================================================

// helpEntry is one row of the help overlay: a section heading when desc is
// empty, otherwise a key and what it does.
type helpEntry struct{ key, desc string }

// helpEntries is the full keymap. Navigation follows lazygit's vocabulary, so
// the keys below are the same ones muscle memory already knows from there.
func helpEntries() []helpEntry {
	return []helpEntry{
		{"Global", ""},
		{"q / Ctrl+c", "Quit"},
		{"?", "Toggle this help"},
		{"1-9", "Switch to view by number"},
		{"Tab / S-Tab", "Next / previous view"},
		{"l / h", "Next / previous view"},
		{"] / [", "Next / previous view"},
		{"P", "Project switcher"},
		{"f", "Favorites picker"},
		{"b", "Branch filter"},
		{"r", "Refresh the active view"},
		{"A", "Reconnect (change host / token)"},

		{"Lists", ""},
		{"j / k", "Down / up"},
		{"↓ / ↑", "Down / up"},
		{". / ,", "Page down / up"},
		{"Ctrl+d / Ctrl+u", "Half page down / up"},
		{"> / <", "Jump to bottom / top"},
		{"G / g", "Jump to bottom / top"},
		{"End / Home", "Jump to bottom / top"},
		{"Enter", "Select / drill in"},
		{"Esc", "Back / cancel"},
		{"o", "Open in browser"},

		{"Pickers (P / b / f)", ""},
		{"/", "Search; Enter applies it, Esc clears"},
		{"f", "Star / unstar the highlighted project"},
		{"Backspace", "Edit the search"},

		{"Pipelines", ""},
		{"Enter", "Jobs, then a job's log"},
		{"p", "Run a new pipeline / play a manual job"},
		{"R", "Retry"},
		{"C", "Cancel"},

		{"Commits & Overview", ""},
		{"Enter", "Jump to the commit's pipeline"},
		{"y", "Copy the commit SHA"},
		{"o", "Open the commit in a browser"},

		{"Merge Requests", ""},
		{"a", "Approve"},
		{"m", "Merge"},

		{"Issues", ""},
		{"c", "Close / reopen"},
	}
}

// handleHelpKey scrolls the help, or closes it on Esc/?/q.
func (a *App) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case KeyEscape, KeyHelp, KeyQuit, KeyEnter:
		a.overlay = overlayNone
		a.helpScroll = 0
		return a, nil
	}

	entries := helpEntries()
	_, boxHeight := a.helpBoxSize(entries)
	if act := components.NavFor(key); act != components.NavNone {
		// The help is scrolled rather than "selected": drive the offset directly.
		a.helpScroll = components.ApplyNav(act, a.helpScroll, len(entries), boxHeight-2)
		return a, nil
	}
	return a, nil
}

// helpBoxSize returns the help overlay's width and height, capped to the screen.
func (a *App) helpBoxSize(entries []helpEntry) (int, int) {
	width := 56
	if width > a.width-2 {
		width = a.width - 2
	}
	// +2 for the borders, +1 for the footer hint.
	height := len(entries) + 3
	if max := a.height - 2; height > max {
		height = max
	}
	if height < 6 {
		height = 6
	}
	return width, height
}

func (a *App) renderHelp() string {
	entries := helpEntries()
	boxWidth, boxHeight := a.helpBoxSize(entries)
	innerWidth := boxWidth - 4
	rows := boxHeight - 3 // content rows, leaving the footer hint

	// Clamp an offset left over from a taller terminal.
	if maxOffset := len(entries) - rows; a.helpScroll > maxOffset {
		a.helpScroll = maxOffset
	}
	if a.helpScroll < 0 {
		a.helpScroll = 0
	}

	var lines []string
	for i := a.helpScroll; i < len(entries) && len(lines) < rows; i++ {
		e := entries[i]
		if e.desc == "" {
			// Section heading, blank line above it except at the very top.
			if len(lines) > 0 && len(lines) < rows-1 {
				lines = append(lines, "")
			}
			lines = append(lines, components.TitleStyle.Render(e.key))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s %s",
			components.HelpKeyStyle.Render(components.PadRight(e.key, 16)),
			components.HelpDescStyle.Render(components.Truncate(e.desc, innerWidth-17)),
		))
	}

	hint := "Esc: close"
	if len(entries) > rows {
		hint = fmt.Sprintf("j/k: scroll (%d-%d of %d)  Esc: close",
			a.helpScroll+1, min(a.helpScroll+rows, len(entries)), len(entries))
	}
	lines = append(lines, components.HelpSepStyle.Render(hint))

	return components.RenderBox("Keybindings", lines, boxWidth, boxHeight,
		components.ColorPrimary, components.ColorPrimary)
}
