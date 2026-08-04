package views

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// Local key constants specific to the MRs view (see pipelines.go for the shared
// subset).
const (
	keyApprove = "a"
	keyMerge   = "m"
)

// MRsView is the self-contained cockpit view for merge requests.
type MRsView struct {
	ctx           *Context
	width, height int // last body size, tracked from tea.WindowSizeMsg / Body

	rowList[gitlab.MergeRequest]

	// detail is the in-place merge-request page, opened with Enter — the same shape as the
	// commit page, so drilling in never moves you to another tab.
	detail mrDetail
}

// NewMRsView creates an MRsView bound to the shared session context.
func NewMRsView(ctx *Context) *MRsView {
	v := &MRsView{ctx: ctx, detail: newMRDetail(ctx)}
	v.match = func(mr gitlab.MergeRequest) string {
		return fmt.Sprintf("!%d %s %s %s", mr.IID, mr.Title, mr.Author, mr.SourceBranch)
	}
	return v
}

// Title implements View.
func (v *MRsView) Title() string { return "Merge Requests" }

// Focus implements View: refreshes what is on screen.
func (v *MRsView) Focus() tea.Cmd {
	if v.detail.active {
		if v.detail.readingBody() {
			return nil
		}
		return v.detail.reload()
	}
	return v.load()
}

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
		v.setItems(msg.MRs)
		return nil

	case tea.PasteMsg:
		if !v.detail.active {
			v.paste(msg.Content)
		}
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}

	// Everything else may belong to the merge-request page or its jobs panel.
	return v.detail.update(msg)
}

// CapturingText implements TextCapturer: while the search is being typed, the shell
// must not read the letters as its own commands.
func (v *MRsView) CapturingText() bool { return !v.detail.active && v.capturing() }

// stepMR moves to the neighbouring merge request, keeping the page open.
func (v *MRsView) stepMR(step int) tea.Cmd {
	visible := v.visible()
	next := v.cursor + step
	if next < 0 || next >= len(visible) {
		return nil // already at an end; the arrow is drawn faint there
	}
	v.cursor = next
	return v.detail.stepAt(v.selected(), v.cursor, len(visible))
}

func (v *MRsView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if v.detail.active {
		// Stepping to the neighbouring merge request belongs to the list's owner, since the
		// page does not know what comes next.
		if step, ok := stepKey(key); ok && !v.detail.readingBody() {
			return v.stepMR(step)
		}
		return v.detail.handleKey(key, v.height)
	}

	if v.navigate(msg, v.height) {
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

// Body implements View: the merge requests, full width, in the same columns every other
// list uses.
func (v *MRsView) Body(width, height int) string {
	v.width = width
	v.height = height

	// Drilled into a merge request: the page takes the whole body.
	if v.detail.active {
		return v.detail.body(width, height)
	}

	return v.box(width, height, "Merge Requests", mrRow, true)
}

// mrRow describes one merge-request row: its number, its CI, what it is, then who wrote
// it, where it comes from, and when it last moved.
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
