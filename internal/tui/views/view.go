package views

import (
	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// ViewID identifies a cockpit view.
type ViewID int

const (
	ViewOverview ViewID = iota
	ViewPipelines
	ViewMRs
	ViewIssues
	ViewCommits
)

// Context is the shared session state handed to every view. Views read it; the
// shell owns and mutates it.
type Context struct {
	Client  *gitlab.Client
	Project *gitlab.Project // nil until selected
	Branch  *gitlab.Branch  // nil = default/all
}

// KeyHint is a footer hint.
type KeyHint struct{ Key, Desc string }

// View is one full-screen cockpit view.
type View interface {
	Focus() tea.Cmd
	Update(msg tea.Msg) tea.Cmd
	Body(width, height int) string
	Title() string
	KeyHints() []KeyHint
}

func viewIDFromName(name string) (ViewID, bool) {
	switch name {
	case "overview":
		return ViewOverview, true
	case "pipelines":
		return ViewPipelines, true
	case "mrs":
		return ViewMRs, true
	case "issues":
		return ViewIssues, true
	case "commits":
		return ViewCommits, true
	}
	return 0, false
}

func defaultViews() []ViewID {
	return []ViewID{ViewOverview, ViewPipelines, ViewMRs, ViewIssues, ViewCommits}
}

// ParseViews converts config names into an ordered, deduplicated ViewID list.
// Empty -> all in default order. Unknown/duplicate dropped with warnings.
func ParseViews(names []string) ([]ViewID, []string) {
	if len(names) == 0 {
		return defaultViews(), nil
	}
	var out []ViewID
	var warnings []string
	seen := make(map[ViewID]bool)
	for _, n := range names {
		id, ok := viewIDFromName(n)
		if !ok {
			warnings = append(warnings, "unknown view \""+n+"\" ignored")
			continue
		}
		if seen[id] {
			warnings = append(warnings, "duplicate view \""+n+"\" ignored")
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return defaultViews(), warnings
	}
	return out, warnings
}

// DefaultViewIndex returns the index of the configured default view within the
// enabled list, or 0 when unset/absent.
func DefaultViewIndex(views []ViewID, name string) int {
	id, ok := viewIDFromName(name)
	if !ok {
		return 0
	}
	for i, v := range views {
		if v == id {
			return i
		}
	}
	return 0
}
