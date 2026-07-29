package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// DashboardView is the project's front page, the way GitLab's own is: what has
// been happening, and what the project says about itself.
//
// The recent commits scroll at the top with their CI status; the README fills the
// space below. Tab moves between them, so both scroll with j/k. The summaries of
// pipelines, merge requests and issues that used to sit here are gone — each has
// a tab of its own, and the README is what you actually want on a front page.
type DashboardView struct {
	ctx           *Context
	width, height int // last body size, tracked from tea.WindowSizeMsg / Body

	commits   []gitlab.Commit
	pipelines []gitlab.Pipeline

	cursor int // into the visible (searched) commits
	scroll int // first visible row, kept across frames

	search listSearch

	// readmeBox is the project's own words, below the commits. It belongs to a
	// project and a ref, so switching either has to drop it.
	readmeBox
	readmeProject int
	readmeRef     string

	// focus says whether the keys drive the commit list or the README.
	focus pageFocus

	// readmeHidden folds the README away, and starts folded: the commits are what
	// the page is opened for, and half the rows spent on prose you have read once is
	// half the commits you cannot see. "t" brings it up. Session-only, so every
	// launch starts on the commits.
	readmeHidden bool

	// detail is the in-place commit page, opened with Enter — the same one the
	// Commits view uses, so drilling in never moves you to another tab.
	detail commitDetail
}

// NewDashboardView creates a DashboardView bound to the shared session context.
func NewDashboardView(ctx *Context) *DashboardView {
	return &DashboardView{ctx: ctx, detail: newCommitDetail(ctx), readmeHidden: true}
}

// Title implements View.
func (v *DashboardView) Title() string { return "Dashboard" }

// Focus implements View: loads commits, pipelines, merge requests, and issues
// concurrently for the active project/branch.
func (v *DashboardView) Focus() tea.Cmd {
	// With a commit page open, the page is what is on screen — refreshing the lists
	// behind it left its pipeline frozen at whatever it said when you opened it.
	if v.detail.active {
		if v.detail.readingBody() {
			return nil
		}
		return v.detail.reload()
	}

	// The commits and the pipelines that give them their CI status are what moves;
	// the README is fetched once per project, since it is the one thing here that
	// does not change while you watch.
	return tea.Batch(v.loadCommits(), v.loadPipelines(), v.syncReadme())
}

// syncReadme fetches the README when it is missing and drops it when it belongs to
// a project or ref you have left — otherwise the last project's words would sit
// under the new project's commits, and a project without a README would keep
// showing someone else's.
func (v *DashboardView) syncReadme() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil {
		return nil
	}
	project, file, ref := v.ctx.Project.ID, v.ctx.Project.ReadmeFile, v.ref()

	if v.readmeProject != project || v.readmeRef != ref {
		v.readmeProject, v.readmeRef = project, ref
		v.resetReadme()
		if file == "" {
			// It has none; saying so beats loading for ever.
			v.setReadme("", "")
			return nil
		}
		return v.loadReadme()
	}
	if v.wants(file) {
		return v.loadReadme()
	}
	return nil
}

// ============================================================================
// Update
// ============================================================================

// Update implements View: navigates the recent-commits list and opens the
// selected commit in a browser. View switching stays with the shell.
func (v *DashboardView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)

	case CommitsLoadedMsg:
		if msg.Err == nil {
			v.commits = msg.Commits
			v.clampCursor()
		}
		return nil

	case PipelinesLoadedMsg:
		if msg.Err == nil {
			v.pipelines = msg.Pipelines
		}
		return nil

	case ReadmeLoadedMsg:
		if msg.Err != nil {
			// A project can deny a raw file (or have moved it) without the dashboard
			// being broken; say so once and stop asking.
			v.setReadme(msg.File, "")
			return statusCmd(fmt.Sprintf("Could not read %s: %v", msg.File, msg.Err), true)
		}
		v.setReadme(msg.File, msg.Source)
		return nil

	case tea.PasteMsg:
		if !v.detail.active {
			v.search.paste(msg.Content, &v.cursor)
		}
		return nil
	}

	// Everything else may belong to the commit page or its jobs panel.
	return v.detail.update(msg)
}

// handleKey navigates the recent-commits list. The same keys work here as in
// every other view, so j/k are never dead — this is the default view.
func (v *DashboardView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if v.detail.active {
		// Stepping to the neighbouring commit belongs to the list's owner, since
		// the page itself does not know what comes next — but not while a diff or a
		// job log is open, where the arrows belong to what you are reading.
		if step, ok := stepKey(key); ok && !v.detail.readingBody() {
			return v.stepCommit(step)
		}
		return v.detail.handleKey(key, v.height)
	}

	switch key {
	case keyToggleBox:
		v.readmeHidden = !v.readmeHidden
		if v.readmeHidden {
			// The keys cannot stay with a box that is no longer on screen.
			v.focus = focusPage
		}
		return nil
	case keyTab:
		v.focus = cycleFocus(v.focus, 1, false, false, !v.readmeHidden)
		return nil
	case keyShiftTab:
		v.focus = cycleFocus(v.focus, -1, false, false, !v.readmeHidden)
		return nil
	}

	// The README has the keys: j/k scroll it, Esc hands them back to the commits.
	if v.focus == focusNotes {
		if key == keyEscape {
			v.focus = focusPage
			return nil
		}
		v.readmeKey(key, v.height)
		return nil
	}

	if v.search.handleKey(msg, &v.cursor) {
		return nil
	}

	if act := components.NavFor(key); act != components.NavNone {
		v.cursor = components.ApplyNav(act, v.cursor, len(v.visible()), listRows(v.height))
		return nil
	}
	if key == keyOpenBrowse {
		return v.openCommitInBrowser()
	}
	if key == keyEnter {
		return v.detail.openAt(v.selectedCommit(), v.cursor, len(v.visible()))
	}
	if key == keyCopy {
		return v.copyHash()
	}
	if key == keyCopyLink {
		if c := v.selectedCommit(); c != nil {
			return copyLink(c.ShortID, c.WebURL)
		}
		return nil
	}
	return nil
}

// CapturingText implements TextCapturer: while the search is being typed, the
// shell must not read the letters as its own commands.
func (v *DashboardView) CapturingText() bool { return !v.detail.active && v.search.capturing() }

// visible is the commits matching the search; the cursor indexes it.
func (v *DashboardView) visible() []gitlab.Commit {
	return filtered(v.commits, v.search.filter, func(c gitlab.Commit) string {
		return c.Title + " " + c.AuthorName + " " + c.ShortID
	})
}

// copyHash copies the selected commit's full SHA to the clipboard, since the
// list shows the author rather than the hash.
func (v *DashboardView) copyHash() tea.Cmd {
	selected := v.selectedCommit()
	if selected == nil {
		return nil
	}
	c := *selected
	sha := c.ID
	if sha == "" {
		sha = c.ShortID
	}
	return tea.Batch(
		copyToClipboard(sha),
		func() tea.Msg { return StatusMsg{Text: "Copied " + c.ShortID + " to the clipboard"} },
	)
}

// stepCommit moves to the neighbouring commit, keeping the page open. It steps
// within the search results when one is applied: the page was opened from that
// list, so those are the commits you are working through.
func (v *DashboardView) stepCommit(step int) tea.Cmd {
	visible := v.visible()
	next := v.cursor + step
	if next < 0 || next >= len(visible) {
		return nil // already at an end; the arrow is drawn faint there
	}
	v.cursor = next
	return v.detail.stepAt(v.selectedCommit(), v.cursor, len(visible))
}

// selectedCommit returns the highlighted commit, or nil.
func (v *DashboardView) selectedCommit() *gitlab.Commit {
	visible := v.visible()
	if v.cursor < 0 || v.cursor >= len(visible) {
		return nil
	}
	return &visible[v.cursor]
}

func (v *DashboardView) clampCursor() {
	v.cursor = clampCursor(v.cursor, len(v.visible()))
}

// openCommitInBrowser opens the selected commit's page on the GitLab host.
func (v *DashboardView) openCommitInBrowser() tea.Cmd {
	c := v.selectedCommit()
	if c == nil {
		return nil
	}
	cmd := openBrowserCmd(c.WebURL)
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		if err := cmd.Start(); err != nil {
			return StatusMsg{Text: "Could not open browser: " + err.Error(), IsErr: true}
		}
		return nil
	}
}

// ============================================================================
// Body / rendering
// ============================================================================

// Body implements View: the recent commits above, the README below.
func (v *DashboardView) Body(width, height int) string {
	v.width = width
	v.height = height

	// Drilled into a commit: the page takes the whole body.
	if v.detail.active {
		return v.detail.body(width, height)
	}

	// Folded away with "t": the commits take the whole page.
	if v.readmeHidden {
		return v.commitsBox(width, height)
	}

	// A blank row between them. Without a frame to do the separating, the two halves
	// otherwise run into each other.
	const gap = 1

	// Half each, which is what makes this a front page rather than a commit list
	// with a footnote. The commits keep a floor, so a short terminal still shows a
	// few of them.
	topHeight := (height - gap) / 2
	if topHeight < 5 {
		topHeight = min(5, height-gap-1)
	}
	bottomHeight := height - gap - topHeight
	if bottomHeight < 1 {
		return v.commitsBox(width, height)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		v.commitsBox(width, topHeight),
		"",
		v.readmePanel(width, bottomHeight, v.focus == focusNotes),
	)
}

// commitsBox renders the recent-commits list.
func (v *DashboardView) commitsBox(width, height int) string {
	visible := v.visible()
	rows := make([]listRow, len(visible))
	for i, c := range visible {
		rows[i] = v.commitRow(c)
	}
	rowWidth := width - components.SelectionGutter
	cols := measureColumns(rows, rowWidth)

	return renderRowsBox(width, height,
		v.search.title("Recent Commits", len(visible), len(v.commits)),
		len(visible),
		func(i int) string { return renderListRow(rows[i], cols, rowWidth) },
		cursorWhen(v.focus == focusPage, v.cursor), &v.scroll)
}

// commitRow describes one recent-commits row, mapping the commit's CI status from
// the loaded pipelines by SHA.
func (v *DashboardView) commitRow(c gitlab.Commit) listRow {
	kind, subject := splitConventional(c.Title)
	return listRow{
		kind:    kind,
		icon:    commitStatusIcon(commitStatus(c.ShortID, v.pipelines)),
		subject: subject,
		author:  c.AuthorName,
		stamp:   commitStamp(c.CreatedAt),
	}
}

// commitStatusIcon renders a commit's CI state, or a faint dot when no pipeline
// ran for it — a blank would break the column that the eye follows down.
func commitStatusIcon(status string) string {
	if status == "" {
		return components.FaintStyle.Render("·") + " "
	}
	return components.StatusIconPadded(status)
}

// commitStatus maps a commit to its CI status by matching a pipeline SHA that
// starts with the commit's ShortID. A success with warnings reports the warning
// pseudo-status, so a failed allowed-to-fail job is not hidden behind a green
// tick — GitLab flags those in its own commit list. Returns "" if none.
func commitStatus(shortID string, pipelines []gitlab.Pipeline) string {
	for _, p := range pipelines {
		if shortID != "" && strings.HasPrefix(p.SHA, shortID) {
			if p.HasWarnings {
				return components.StatusWarning
			}
			return p.Status
		}
	}
	return ""
}

// ============================================================================
// KeyHints
// ============================================================================

// KeyHints implements View.
func (v *DashboardView) KeyHints() []KeyHint {
	if v.detail.active {
		return v.detail.keyHints()
	}
	if v.focus == focusNotes {
		hints := []KeyHint{}
		if v.scrollable {
			hints = append(hints, KeyHint{"j/k", "Scroll"})
		}
		return append(hints,
			KeyHint{"Tab", "Commits"}, KeyHint{"t", "Hide readme"}, KeyHint{"Esc", "Back"})
	}
	hints := []KeyHint{{Key: "Enter", Desc: "Commit page"}}
	// The hint says which way the key goes, so it never promises a box that is
	// already there or offers to hide one that is not.
	if v.readmeHidden {
		hints = append(hints, KeyHint{Key: "t", Desc: "Show readme"})
	} else {
		hints = append(hints, KeyHint{Key: "Tab", Desc: "Readme"}, KeyHint{Key: "t", Desc: "Hide readme"})
	}
	return append(hints,
		KeyHint{Key: "y/Y", Desc: "Copy SHA/link"},
		KeyHint{Key: "o", Desc: "Open commit"},
		v.search.hint(),
	)
}

// ============================================================================
// Commands (async API calls)
// ============================================================================

func (v *DashboardView) ref() string {
	if v.ctx == nil || v.ctx.Branch == nil {
		return ""
	}
	return v.ctx.Branch.Name
}

func (v *DashboardView) loadCommits() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	ref := v.ref()
	return func() tea.Msg {
		commits, err := client.ListCommits(projectID, ref)
		return CommitsLoadedMsg{Commits: commits, Err: err}
	}
}

func (v *DashboardView) loadPipelines() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	if v.ctx.Branch != nil {
		ref := v.ctx.Branch.Name
		return func() tea.Msg {
			pipelines, err := client.ListPipelinesByRef(projectID, ref)
			return PipelinesLoadedMsg{Pipelines: pipelines, Err: err}
		}
	}
	return func() tea.Msg {
		pipelines, err := client.ListPipelines(projectID)
		return PipelinesLoadedMsg{Pipelines: pipelines, Err: err}
	}
}

// loadReadme fetches the project's README once per project.
func (v *DashboardView) loadReadme() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	file := v.ctx.Project.ReadmeFile
	ref := v.ref()
	return func() tea.Msg {
		source, err := client.GetReadme(projectID, file, ref)
		return ReadmeLoadedMsg{File: file, Source: source, Err: err}
	}
}
