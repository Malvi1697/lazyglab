package tui

import "fmt"

// panelIDFromName maps a config name to a PanelID.
func panelIDFromName(name string) (PanelID, bool) {
	switch name {
	case "projects":
		return PanelProjects, true
	case "pipelines":
		return PanelPipelines, true
	case "merge_requests":
		return PanelMergeRequests, true
	case "issues":
		return PanelIssues, true
	}
	return 0, false
}

// defaultPanels is the full ordered set of panels.
func defaultPanels() []PanelID {
	return []PanelID{PanelProjects, PanelPipelines, PanelMergeRequests, PanelIssues}
}

// ParsePanels converts config panel names into an ordered, deduplicated list of
// visible panels. Rules: empty input -> all panels in default order; unknown and
// duplicate names are dropped (with a warning); the Projects panel is always
// present (prepended if the user omitted it). Returns the panels and warnings.
func ParsePanels(names []string) ([]PanelID, []string) {
	if len(names) == 0 {
		return defaultPanels(), nil
	}

	var out []PanelID
	var warnings []string
	seen := make(map[PanelID]bool)

	for _, n := range names {
		id, ok := panelIDFromName(n)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("unknown panel %q ignored", n))
			continue
		}
		if seen[id] {
			warnings = append(warnings, fmt.Sprintf("duplicate panel %q ignored", n))
			continue
		}
		seen[id] = true
		out = append(out, id)
	}

	if !seen[PanelProjects] {
		out = append([]PanelID{PanelProjects}, out...)
		warnings = append(warnings, "projects panel is required; added automatically")
	}

	return out, warnings
}
