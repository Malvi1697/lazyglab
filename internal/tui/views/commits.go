package views

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/util"
)

// CommitsView is the self-contained cockpit view for commits.
type CommitsView struct {
	ctx           *Context
	width, height int // last body size, tracked from tea.WindowSizeMsg / Body

	commits []gitlab.Commit
	cursor  int // indexes the visible (searched) list, not commits
	scroll  int // first visible row, kept across frames

	search listSearch

	// detail is the in-place commit page, opened with Enter.
	detail commitDetail
}

// NewCommitsView creates a CommitsView bound to the shared session context.
func NewCommitsView(ctx *Context) *CommitsView {
	return &CommitsView{ctx: ctx, detail: newCommitDetail(ctx)}
}

// Title implements View.
func (v *CommitsView) Title() string { return "Commits" }

// Focus implements View: loads commits for the active project/branch.
func (v *CommitsView) Focus() tea.Cmd { return v.load() }

// ============================================================================
// Update
// ============================================================================

// Update implements View.
func (v *CommitsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return nil

	case CommitsLoadedMsg:
		if msg.Err != nil {
			return statusCmd(fmt.Sprintf("Error loading commits: %v", msg.Err), true)
		}
		v.commits = msg.Commits
		v.clampCursor()
		return nil

	case tea.PasteMsg:
		if !v.detail.active {
			v.search.paste(msg.Content, &v.cursor)
		}
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}

	// Everything else may belong to the commit page or its jobs panel.
	return v.detail.update(msg)
}

func (v *CommitsView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if v.detail.active {
		// Stepping to the neighbouring commit belongs to the list's owner, since
		// the page itself does not know what comes next — but not while a diff or a
		// job log is open, where the arrows belong to what you are reading.
		if step, ok := commitStep(key); ok && !v.detail.readingBody() {
			return v.stepCommit(step)
		}
		return v.detail.handleKey(key, v.height)
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
		return v.detail.openAt(v.selected(), v.cursor, len(v.visible()))
	}
	if key == keyCopy {
		return v.copyHash()
	}
	if key == keyCopyLink {
		if c := v.selected(); c != nil {
			return copyLink(c.ShortID, c.WebURL)
		}
		return nil
	}
	return nil
}

// CapturingText implements TextCapturer: while the search is being typed, the
// shell must not read the letters as its own commands.
func (v *CommitsView) CapturingText() bool { return !v.detail.active && v.search.capturing() }

// visible is the commits matching the search; the cursor indexes it.
func (v *CommitsView) visible() []gitlab.Commit {
	return filtered(v.commits, v.search.filter, func(c gitlab.Commit) string {
		return c.Title + " " + c.AuthorName + " " + c.ShortID
	})
}

// openDetail drills into the selected commit: its full message plus the pipelines
// GitLab ran for it. Unlike jumping straight to the Pipelines view, this stays
// put and is meaningful even when no pipeline ever ran for the commit.
// handleDetailKey drives the commit detail. Esc goes back to the list.
// loadDetail fetches the commit's full message and its pipelines.
// commitPipeline returns the most recent pipeline for the shown commit, or nil.
// retryCommitPipeline retries the commit's pipeline, if it has one.
// runPipelineOnRef runs a new pipeline on the active ref.
//
// GitLab creates pipelines for a ref, never for an arbitrary commit, so this
// builds the ref's current head — which is only this commit if it happens to be
// the tip. The confirmation says so instead of implying otherwise.
// focusPending moves the cursor to pendingSHA and reports whether it was found.
// stepCommit moves to the neighbouring commit, keeping the page open.
func (v *CommitsView) stepCommit(step int) tea.Cmd {
	visible := v.visible()
	next := v.cursor + step
	if next < 0 || next >= len(visible) {
		return nil // already at an end; the arrow is drawn faint there
	}
	v.cursor = next
	return v.detail.openAt(v.selected(), v.cursor, len(visible))
}

// selected returns the highlighted commit, or nil.
func (v *CommitsView) selected() *gitlab.Commit {
	visible := v.visible()
	if v.cursor < 0 || v.cursor >= len(visible) {
		return nil
	}
	return &visible[v.cursor]
}

// copyHash copies the selected commit's full SHA to the clipboard. The list
// shows the author rather than the hash, so this is how the hash is obtained.
func (v *CommitsView) copyHash() tea.Cmd {
	selected := v.selected()
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

// showPipeline asks the shell to open the selected commit's pipeline.
// ============================================================================
// Body / rendering
// ============================================================================

// Body implements View: a horizontal split with a list on the left and a
// detail panel on the right.
func (v *CommitsView) Body(width, height int) string {
	v.width = width
	v.height = height

	// Drilled into a commit: the page gets the whole body, like GitLab's own
	// commit page, rather than being squeezed into the detail pane.
	if v.detail.active {
		return v.detail.body(width, height)
	}

	leftWidth := width * 45 / 100
	if leftWidth < 20 {
		leftWidth = 20
	}
	if leftWidth > width {
		leftWidth = width
	}
	rightWidth := width - leftWidth

	visible := v.visible()
	left := renderRowsBox(leftWidth, height,
		v.search.title("Commits", len(visible), len(v.commits)),
		len(visible), func(i int) string { return commitItemRow(visible[i]) },
		v.cursor, &v.scroll)

	detail := v.commitDetail()
	if detail == "" {
		detail = "Select an item to view details"
	}
	right := components.RenderPanel("Commit", splitLines(detail), rightWidth-4, height, false)

	return joinPanels(left, right, height)
}

// commitItemRow renders one commit list row.
func commitItemRow(c gitlab.Commit) string {
	return commitRow(util.CommitTime(c.CreatedAt), commitStatusIcon(c.Status),
		c.AuthorName, c.Title)
}

func (v *CommitsView) commitDetail() string {
	if len(v.commits) == 0 {
		return "No commits"
	}
	c := v.selected()
	if c == nil {
		return ""
	}

	return fmt.Sprintf("%s\n\n%s\n\nAuthor: %s\n%s\n\n%s\n\n%s",
		components.TitleStyle.Render(c.ShortID),
		c.Title,
		c.AuthorName,
		util.TimeAgo(c.CreatedAt),
		components.HelpDescStyle.Render(c.WebURL),
		components.HelpDescStyle.Render("Enter: commit detail & pipelines"),
	)
}

// commitDetailFull renders the drilled-in commit the way GitLab's commit page
// does: the message, then what the commit belongs to (parent, branches, merge
// requests) and the pipelines it triggered.
// detailRefsLine lists the branches and tags containing the commit.
// detailMRLines lists the merge requests the commit belongs to.
// detailPipelineLines renders the pipelines run for the commit, distinguishing a
// success with warnings from a plain success — that is the whole reason the
// detail asks GitLab for each pipeline individually.
// shortSHA abbreviates a full SHA the way GitLab displays it.
func shortSHA(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}

// ============================================================================
// KeyHints
// ============================================================================

// KeyHints implements View.
func (v *CommitsView) KeyHints() []KeyHint {
	if v.detail.active {
		return v.detail.keyHints()
	}
	return []KeyHint{
		{"Enter", "Commit page"},
		{"y/Y", "Copy SHA/link"},
		{"o", "Open"},
		v.search.hint(),
	}
}

// ============================================================================
// Commands (async API calls)
// ============================================================================

// ref is the active branch, empty when the project default is in use.
func (v *CommitsView) ref() string {
	if v.ctx == nil || v.ctx.Branch == nil {
		return ""
	}
	return v.ctx.Branch.Name
}

func (v *CommitsView) load() tea.Cmd {
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

func (v *CommitsView) openCommitInBrowser() tea.Cmd {
	c := v.selected()
	if c == nil {
		return nil
	}
	cmd := openBrowserCmd(c.WebURL)
	if cmd == nil {
		return nil
	}
	return execBrowser(cmd)
}

// ============================================================================
// Helpers
// ============================================================================

func (v *CommitsView) clampCursor() {
	v.cursor = clampCursor(v.cursor, len(v.visible()))
}
