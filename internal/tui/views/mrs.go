package views

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/util"
)

// Local key constants specific to the MRs view (see pipelines.go for the
// shared subset).
const (
	keyApprove = "a"
	keyMerge   = "m"
)

// MRsView is the self-contained cockpit view for merge requests.
type MRsView struct {
	ctx           *Context
	width, height int // last body size, tracked from tea.WindowSizeMsg / Body

	mrs    []gitlab.MergeRequest
	cursor int // indexes the visible (searched) list, not mrs
	scroll int // first visible row, kept across frames

	search listSearch

	// detail is the in-place merge-request page, opened with Enter — the same shape
	// as the commit page, so drilling in never moves you to another tab.
	detail mrDetail
}

// NewMRsView creates an MRsView bound to the shared session context.
func NewMRsView(ctx *Context) *MRsView {
	return &MRsView{ctx: ctx, detail: newMRDetail(ctx)}
}

// Title implements View.
func (v *MRsView) Title() string { return "Merge Requests" }

// Focus implements View: refreshes what is on screen. With a merge request open,
// that is the page — its pipeline, its jobs, its approvals — and not the list
// behind it, which was the reason a page you sat watching never changed.
//
// Anything long-form being read (a diff, a log, a thread) is left alone: a refetch
// would move what someone is halfway through.
func (v *MRsView) Focus() tea.Cmd {
	if v.detail.active {
		if v.detail.readingBody() {
			return nil
		}
		return v.detail.reload()
	}
	return v.load()
}

// ============================================================================
// Update
// ============================================================================

// Update implements View.
func (v *MRsView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return nil

	case MRsLoadedMsg:
		if msg.Err != nil {
			return statusCmd(fmt.Sprintf("Error loading merge requests: %v", msg.Err), true)
		}
		v.mrs = msg.MRs
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

	// Everything else may belong to the merge-request page or its jobs panel.
	return v.detail.update(msg)
}

// CapturingText implements TextCapturer: while the search is being typed, the
// shell must not read the letters as its own commands.
func (v *MRsView) CapturingText() bool { return !v.detail.active && v.search.capturing() }

// stepMR moves to the neighbouring merge request, keeping the page open. It steps
// within the search results when one is applied: the page was opened from that
// list, so those are the ones you are working through.
func (v *MRsView) stepMR(step int) tea.Cmd {
	visible := v.visible()
	next := v.cursor + step
	if next < 0 || next >= len(visible) {
		return nil // already at an end; the arrow is drawn faint there
	}
	v.cursor = next
	return v.detail.stepAt(v.selected(), v.cursor, len(visible))
}

// visible is the merge requests matching the search; the cursor indexes it.
func (v *MRsView) visible() []gitlab.MergeRequest {
	return filtered(v.mrs, v.search.filter, func(mr gitlab.MergeRequest) string {
		return fmt.Sprintf("!%d %s %s %s", mr.IID, mr.Title, mr.Author, mr.SourceBranch)
	})
}

// selected returns the highlighted merge request, or nil.
func (v *MRsView) selected() *gitlab.MergeRequest {
	visible := v.visible()
	if v.cursor < 0 || v.cursor >= len(visible) {
		return nil
	}
	return &visible[v.cursor]
}

func (v *MRsView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if v.detail.active {
		// Stepping to the neighbouring merge request belongs to the list's owner,
		// since the page does not know what comes next — but not while a diff or a
		// job log is open, where the arrows belong to what you are reading.
		if step, ok := stepKey(key); ok && !v.detail.readingBody() {
			return v.stepMR(step)
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

	mr := v.selected()
	if mr == nil {
		return nil
	}
	switch key {
	case keyEnter:
		return v.detail.openAt(mr, v.cursor, len(v.visible()))
	case keyApprove:
		return confirmCmd(fmt.Sprintf("Approve !%d %s?", mr.IID, mr.Title), v.approveMR())
	case keyMerge:
		return confirmCmd(fmt.Sprintf("Merge !%d %s?", mr.IID, mr.Title), v.mergeMR())
	case keyCopy:
		return copyRef(fmt.Sprintf("!%d", mr.IID))
	case keyCopyLink:
		return copyLink(fmt.Sprintf("!%d", mr.IID), mr.WebURL)
	case keyOpenBrowse:
		if cmd := openBrowserCmd(mr.WebURL); cmd != nil {
			return execBrowser(cmd)
		}
	}
	return nil
}

// ============================================================================
// Body / rendering
// ============================================================================

// Body implements View: a horizontal split with a list on the left and a
// detail panel on the right.
func (v *MRsView) Body(width, height int) string {
	v.width = width
	v.height = height

	// Drilled into a merge request: the page takes the whole body.
	if v.detail.active {
		return v.detail.body(width, height)
	}

	// The list carries five columns now, so it gets the larger share: the detail
	// beside it is a preview of what Enter opens properly.
	leftWidth := width * 62 / 100
	if leftWidth < 20 {
		leftWidth = 20
	}
	if leftWidth > width {
		leftWidth = width
	}
	rightWidth := width - leftWidth

	visible := v.visible()
	rowWidth := leftWidth - components.SelectionGutter
	titleWidth := mrTitleWidth(visible, rowWidth)
	left := renderRowsBox(leftWidth, height,
		v.search.title("Merge Requests", len(visible), len(v.mrs)),
		len(visible), func(i int) string { return mrRow(visible[i], titleWidth, rowWidth) },
		v.cursor, &v.scroll)

	detail := v.mrDetail()
	if detail == "" {
		detail = "Select an item to view details"
	}
	right := components.RenderPanel(v.detailTitle(), strings.Split(detail, "\n"), rightWidth-4, height, false)

	return joinPanels(left, right, height)
}

func (v *MRsView) detailTitle() string {
	if mr := v.selected(); mr != nil {
		return fmt.Sprintf("MR (!%d)", mr.IID)
	}
	return "Merge Request"
}

// The merge-request row's columns: its number, its CI, what it is, and then who,
// from where, and when — the same shape GitLab's own list has.
//
// The last three are dropped as the terminal narrows, widest first: what it is
// matters more than who wrote it, and a truncated branch name says nothing.
const (
	mrRefWidth     = 6
	mrAuthorWidth  = 14
	mrBranchWidth  = 20
	mrUpdatedWidth = 8 // util.CommitTime's column
	mrTitleMin     = 24
)

// mrTitleWidth measures the title column over the whole list, so the columns after
// it stay beside it instead of against the right edge.
func mrTitleWidth(mrs []gitlab.MergeRequest, width int) int {
	titles := make([]string, len(mrs))
	for i, mr := range mrs {
		titles[i] = mr.Title
		if mr.Draft {
			titles[i] = "[Draft] " + titles[i]
		}
	}
	// The title and the author are what a row is for, so they get their place first.
	room := width - mrRefWidth - 1 - 2 - 1 - (1 + mrAuthorWidth)

	// The branch and the date keep theirs too, when the row is wide enough to hold
	// all four: one outlier of a title would otherwise eat the columns everybody
	// else's row would have shown, and a column a single row can delete is not a
	// column. On a narrower row they drop off instead, and the title takes the space.
	if optional := (1 + mrBranchWidth) + (1 + mrUpdatedWidth); room-optional >= mrTitleMin {
		room -= optional
	}
	return columnWidth(titles, mrTitleMin, max(room, mrTitleMin))
}

// mrRow renders one merge-request list row within width.
func mrRow(mr gitlab.MergeRequest, titleWidth, width int) string {
	title := mr.Title
	if mr.Draft {
		title = "[Draft] " + title
	}

	icon := components.StatusIconPadded("") // two cells of nothing, so the column holds
	if mr.Pipeline != nil {
		icon = components.StatusIconPadded(mr.Pipeline.Status)
	}
	head := components.MutedStyle.Render(
		components.PadRight(fmt.Sprintf("!%d", mr.IID), mrRefWidth)) + " " + icon

	row := head + " " +
		components.BodyStyle.Render(components.PadRight(components.Truncate(title, titleWidth), titleWidth))

	// Every column that still fits, in the order you would give them up: what it is
	// matters more than who wrote it, and a branch cut to eight characters says
	// nothing.
	for _, col := range []struct {
		width int
		text  string
	}{
		{mrAuthorWidth, mr.Author},
		{mrBranchWidth, mr.SourceBranch},
		{mrUpdatedWidth, updatedStamp(mr.UpdatedAt)},
	} {
		if lipgloss.Width(row)+1+col.width > width {
			break
		}
		row += " " + components.MutedStyle.Render(
			components.PadRight(components.Truncate(col.text, col.width), col.width))
	}
	return row
}

// updatedStamp is when something last moved, in the same words a commit row uses.
func updatedStamp(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return strings.TrimSpace(util.CommitTime(t))
}

func (v *MRsView) mrDetail() string {
	if len(v.mrs) == 0 {
		return "No merge requests"
	}
	mr := v.selected()
	if mr == nil {
		if v.search.on() {
			return components.HelpDescStyle.Render("No merge request matches " + v.search.filter.Query)
		}
		return ""
	}

	pipeStatus := "none"
	if mr.Pipeline != nil {
		pipeStatus = mr.Pipeline.Status
	}
	draft := ""
	if mr.Draft {
		draft = " [Draft]"
	}

	return fmt.Sprintf("%s%s\n\n%s -> %s\nAuthor: %s\nPipeline: %s\n\n%s\n\n%s\n\n%s",
		components.TitleStyle.Render(fmt.Sprintf("!%d %s", mr.IID, mr.Title)),
		draft,
		mr.SourceBranch, mr.TargetBranch,
		mr.Author,
		pipeStatus,
		mr.Description,
		components.HelpDescStyle.Render(mr.WebURL),
		components.HelpDescStyle.Render("Enter: the full merge-request page"),
	)
}

// ============================================================================
// KeyHints
// ============================================================================

// KeyHints implements View.
func (v *MRsView) KeyHints() []KeyHint {
	if v.detail.active {
		return v.detail.keyHints()
	}
	return []KeyHint{
		{"Enter", "MR page"},
		{"a", "Approve"},
		{"m", "Merge"},
		{"o", "Open"},
		{"y/Y", "Copy !/link"},
		v.search.hint(),
	}
}

// ============================================================================
// Commands (async API calls)
// ============================================================================

func (v *MRsView) load() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	return func() tea.Msg {
		mrs, err := client.ListMergeRequests(projectID)
		return MRsLoadedMsg{MRs: mrs, Err: err}
	}
}

func (v *MRsView) approveMR() tea.Cmd {
	mr := v.selected()
	if mr == nil || v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	mrIID := mr.IID
	return func() tea.Msg {
		if err := client.ApproveMergeRequest(projectID, mrIID); err != nil {
			return StatusMsg{Text: fmt.Sprintf("Approve failed: %v", err), IsErr: true}
		}
		return StatusMsg{Text: fmt.Sprintf("Approved !%d", mrIID)}
	}
}

func (v *MRsView) mergeMR() tea.Cmd {
	mr := v.selected()
	if mr == nil || v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	mrIID := mr.IID
	return func() tea.Msg {
		if err := client.MergeMergeRequest(projectID, mrIID); err != nil {
			return StatusMsg{Text: fmt.Sprintf("Merge failed: %v", err), IsErr: true}
		}
		return StatusMsg{Text: fmt.Sprintf("Merged !%d", mrIID)}
	}
}

// ============================================================================
// Helpers
// ============================================================================

func (v *MRsView) clampCursor() {
	v.cursor = clampCursor(v.cursor, len(v.visible()))
}
