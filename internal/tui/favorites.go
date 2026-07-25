package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/tui/views"
)

// SaveFavoritesFunc persists the starred project paths of a host. The app
// package injects it, because config handling lives there.
type SaveFavoritesFunc func(host string, favorites []string) error

// SaveLastProjectFunc persists the project last selected on a host, so the next
// launch resumes there. Injected by the app package for the same reason.
type SaveLastProjectFunc func(host, path string) error

// favoritesSavedMsg reports the outcome of writing favorites to the config.
type favoritesSavedMsg struct{ err error }

// isFavorite reports whether a project path is starred.
func (a *App) isFavorite(path string) bool {
	return slices.Contains(a.favorites, path)
}

// toggleFavorite stars or unstars a project path and persists the result.
func (a *App) toggleFavorite(path string) tea.Cmd {
	if path == "" {
		return nil
	}
	if i := slices.Index(a.favorites, path); i >= 0 {
		a.favorites = slices.Delete(a.favorites, i, i+1)
		a.favoritesStatus = "Removed from favorites: " + path
	} else {
		a.favorites = append(a.favorites, path)
		a.favoritesStatus = "Added to favorites: " + path
	}
	a.clampFavoriteCursor()

	if a.saveFavorites == nil {
		return nil
	}
	host := a.activeHost
	// Persist a copy: the slice keeps being mutated while the write runs.
	favorites := slices.Clone(a.favorites)
	save := a.saveFavorites
	return func() tea.Msg { return favoritesSavedMsg{err: save(host, favorites)} }
}

// lastProjectSavedMsg reports the outcome of recording the active project.
type lastProjectSavedMsg struct{ err error }

// rememberProject records the active project so the next launch resumes it.
// Unchanged selections write nothing.
func (a *App) rememberProject(path string) tea.Cmd {
	if path == "" || path == a.lastProject || a.saveLastProject == nil {
		return nil
	}
	a.lastProject = path
	host := a.activeHost
	save := a.saveLastProject
	return func() tea.Msg { return lastProjectSavedMsg{err: save(host, path)} }
}

// clampFavoriteCursor keeps the favorites cursor within bounds.
func (a *App) clampFavoriteCursor() {
	if a.favoriteCursor >= len(a.favorites) {
		a.favoriteCursor = len(a.favorites) - 1
	}
	if a.favoriteCursor < 0 {
		a.favoriteCursor = 0
	}
}

// openFavorites shows the favorites picker.
func (a *App) openFavorites() {
	a.clampFavoriteCursor()
	a.favoritesStatus = ""
	a.overlay = overlayFavorites
}

// handleFavoritesKey drives the favorites picker.
func (a *App) handleFavoritesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch {
	case key == KeyEscape || key == KeyQuit:
		a.overlay = overlayNone
		return a, nil
	case isNavigateUp(msg):
		if a.favoriteCursor > 0 {
			a.favoriteCursor--
		}
		return a, nil
	case isNavigateDown(msg):
		if a.favoriteCursor < len(a.favorites)-1 {
			a.favoriteCursor++
		}
		return a, nil
	case key == KeyTop:
		a.favoriteCursor = 0
		return a, nil
	case key == KeyBottom:
		a.favoriteCursor = len(a.favorites) - 1
		a.clampFavoriteCursor()
		return a, nil
	case key == KeyFavorite:
		// Unstar from inside the picker, so cleaning up needs no detour.
		if path := a.selectedFavorite(); path != "" {
			return a, a.toggleFavorite(path)
		}
		return a, nil
	case key == KeyEnter:
		if path := a.selectedFavorite(); path != "" {
			a.overlay = overlayNone
			return a, a.selectProjectByPath(path)
		}
		return a, nil
	}
	return a, nil
}

// selectedFavorite returns the highlighted favorite path, or "".
func (a *App) selectedFavorite() string {
	if a.favoriteCursor < 0 || a.favoriteCursor >= len(a.favorites) {
		return ""
	}
	return a.favorites[a.favoriteCursor]
}

// selectProjectByPath activates a project identified by path. Already-loaded
// projects are used directly; anything else is fetched, since ListProjects only
// returns the 50 most recently active projects and a favorite is often outside
// that set — which is much of the point of having favorites.
func (a *App) selectProjectByPath(path string) tea.Cmd {
	for _, p := range a.projects {
		if strings.EqualFold(p.PathWithNamespace, path) {
			proj := p
			return func() tea.Msg { return views.ProjectSelectedMsg{Project: proj} }
		}
	}

	client := a.ctx.Client
	if client == nil {
		return nil
	}
	a.setStatus("Loading "+path+"...", false)
	return func() tea.Msg {
		project, err := client.GetProjectByPath(path)
		if err != nil {
			return views.StatusMsg{Text: fmt.Sprintf("Cannot open %s: %v", path, err), IsErr: true}
		}
		return views.ProjectSelectedMsg{Project: *project}
	}
}

// renderFavorites draws the favorites picker.
func (a *App) renderFavorites() string {
	boxWidth, boxHeight := a.overlayBoxSize()
	maxVisible := boxHeight - 4
	if maxVisible < 3 {
		maxVisible = 3
	}
	innerWidth := boxWidth - 4

	var lines []string
	if len(a.favorites) == 0 {
		lines = append(lines,
			components.HelpDescStyle.Render("No favorites yet."),
			"",
			components.HelpDescStyle.Render("Press P, highlight a project and press f to star it."),
		)
	} else {
		a.favoriteScroll = components.ScrollOffset(a.favoriteScroll, a.favoriteCursor, len(a.favorites), maxVisible)
		for i := a.favoriteScroll; i < len(a.favorites) && len(lines) < maxVisible; i++ {
			path := a.favorites[i]
			label := components.Truncate("★ "+a.favoriteLabel(path), innerWidth)
			if i == a.favoriteCursor {
				label = components.SelectedItemStyle.Render(components.PadRight(label, innerWidth))
			}
			lines = append(lines, label)
		}
	}

	lines = append(lines, "")
	if a.favoritesStatus != "" {
		lines = append(lines, components.HelpDescStyle.Render(components.Truncate(a.favoritesStatus, innerWidth)))
	} else {
		lines = append(lines, components.HelpDescStyle.Render("Enter: open  f: unstar  Esc: close  j/k: navigate"))
	}

	return components.RenderBox("Favorites", lines, boxWidth, boxHeight, components.ColorPrimary, components.ColorPrimary)
}

// favoriteLabel shows the friendly project name when it is already loaded,
// falling back to the stored path.
func (a *App) favoriteLabel(path string) string {
	for _, p := range a.projects {
		if strings.EqualFold(p.PathWithNamespace, path) {
			return p.NameWithNamespace
		}
	}
	return path
}
