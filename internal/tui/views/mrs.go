package views

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
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

// Body implements View: the merge requests, full width, in the same columns every
// other list uses.
//
// The preview panel beside the list is gone: Enter opens the merge-request page
// itself, which says everything the preview did and more, and the width it took
// was coming out of the titles — the one thing on the row that cannot be guessed.
func (v *MRsView) Body(width, height int) string {
	v.width = width
	v.height = height

	// Drilled into a merge request: the page takes the whole body.
	if v.detail.active {
		return v.detail.body(width, height)
	}

	visible := v.visible()
	rows := make([]listRow, len(visible))
	for i, mr := range visible {
		rows[i] = mrRow(mr)
	}
	rowWidth := width - components.SelectionGutter
	cols := measureColumns(rows, rowWidth)

	return renderRowsBox(width, height,
		v.search.title("Merge Requests", len(visible), len(v.mrs)),
		len(visible), func(i int) string { return renderListRow(rows[i], cols, rowWidth) },
		v.cursor, &v.scroll)
}

// mrRow describes one merge-request row: its number, its CI, what it is, then who
// wrote it, where it comes from, and when it last moved — the same shape GitLab's
// own list has.
func mrRow(mr gitlab.MergeRequest) listRow {
	title := mr.Title
	if mr.Draft {
		title = "[Draft] " + title
	}
	kind, subject := splitConventional(title)

	icon := components.StatusIconPadded("") // two cells of nothing, so the column holds
	if mr.Pipeline != nil {
		icon = components.StatusIconPadded(mr.Pipeline.Status)
	}

	return listRow{
		ref:     fmt.Sprintf("!%d", mr.IID),
		kind:    kind,
		icon:    icon,
		subject: subject,
		author:  mr.Author,
		extra:   mr.SourceBranch,
		stamp:   commitStamp(mr.UpdatedAt),
	}
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
