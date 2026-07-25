package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

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
	cursor int
	scroll int // first visible row, kept across frames

	status string
}

// NewMRsView creates an MRsView bound to the shared session context.
func NewMRsView(ctx *Context) *MRsView { return &MRsView{ctx: ctx} }

// Title implements View.
func (v *MRsView) Title() string { return "Merge Requests" }

// Focus implements View: loads merge requests for the active project.
func (v *MRsView) Focus() tea.Cmd { return v.load() }

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
			v.status = fmt.Sprintf("Error loading merge requests: %v", msg.Err)
			return nil
		}
		v.mrs = msg.MRs
		v.clampCursor()
		v.status = fmt.Sprintf("Loaded %d merge requests", len(msg.MRs))
		return nil

	case StatusMsg:
		v.status = msg.Text
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}
	return nil
}

func (v *MRsView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if isNavUp(msg) {
		v.moveCursor(-1)
		return nil
	}
	if isNavDown(msg) {
		v.moveCursor(1)
		return nil
	}
	if key == keyTop {
		v.cursor = 0
		return nil
	}
	if key == keyBottom {
		v.cursor = len(v.mrs) - 1
		if v.cursor < 0 {
			v.cursor = 0
		}
		return nil
	}
	if key == keyHalfDown {
		v.moveCursor(halfPage(v.height))
		return nil
	}
	if key == keyHalfUp {
		v.moveCursor(-halfPage(v.height))
		return nil
	}

	switch key {
	case keyApprove:
		if v.cursor < len(v.mrs) {
			mr := v.mrs[v.cursor]
			return confirmCmd(fmt.Sprintf("Approve !%d %s?", mr.IID, mr.Title), v.approveMR())
		}
	case keyMerge:
		if v.cursor < len(v.mrs) {
			mr := v.mrs[v.cursor]
			return confirmCmd(fmt.Sprintf("Merge !%d %s?", mr.IID, mr.Title), v.mergeMR())
		}
	case keyOpenBrowse:
		return v.openMRInBrowser()
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

	leftWidth := width * 45 / 100
	if leftWidth < 20 {
		leftWidth = 20
	}
	if leftWidth > width {
		leftWidth = width
	}
	rightWidth := width - leftWidth

	left := renderListBox(leftWidth, height, "Merge Requests", v.mrItems(), v.cursor, &v.scroll)

	detail := v.mrDetail()
	if detail == "" {
		detail = "Select an item to view details"
	}
	right := components.RenderBox(v.detailTitle(), strings.Split(detail, "\n"), rightWidth, height, components.ColorSecondary, components.ColorPrimary)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (v *MRsView) detailTitle() string {
	if v.cursor < len(v.mrs) {
		return fmt.Sprintf("MR (!%d)", v.mrs[v.cursor].IID)
	}
	return "Merge Request"
}

// mrItems renders the MR list rows.
func (v *MRsView) mrItems() []string {
	items := make([]string, len(v.mrs))
	for i, mr := range v.mrs {
		prefix := ""
		if mr.Draft {
			prefix = "[Draft] "
		}
		pipeIcon := ""
		if mr.Pipeline != nil {
			pipeIcon = " " + components.StatusIcon(mr.Pipeline.Status)
		}
		items[i] = fmt.Sprintf("!%d %s%s%s", mr.IID, prefix, mr.Title, pipeIcon)
	}
	return items
}

func (v *MRsView) mrDetail() string {
	if len(v.mrs) == 0 {
		return "No merge requests"
	}
	if v.cursor >= len(v.mrs) {
		return ""
	}
	mr := v.mrs[v.cursor]

	pipeStatus := "none"
	if mr.Pipeline != nil {
		pipeStatus = mr.Pipeline.Status
	}
	draft := ""
	if mr.Draft {
		draft = " [Draft]"
	}

	return fmt.Sprintf("%s%s\n\n%s -> %s\nAuthor: %s\nPipeline: %s\n\n%s\n\n%s",
		components.TitleStyle.Render(fmt.Sprintf("!%d %s", mr.IID, mr.Title)),
		draft,
		mr.SourceBranch, mr.TargetBranch,
		mr.Author,
		pipeStatus,
		mr.Description,
		components.HelpDescStyle.Render(mr.WebURL),
	)
}

// ============================================================================
// KeyHints
// ============================================================================

// KeyHints implements View.
func (v *MRsView) KeyHints() []KeyHint {
	return []KeyHint{
		{"a", "Approve"},
		{"m", "Merge"},
		{"o", "Open"},
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
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil || len(v.mrs) == 0 {
		return nil
	}
	if v.cursor >= len(v.mrs) {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	mrIID := v.mrs[v.cursor].IID
	return func() tea.Msg {
		if err := client.ApproveMergeRequest(projectID, mrIID); err != nil {
			return StatusMsg{Text: fmt.Sprintf("Approve failed: %v", err), IsErr: true}
		}
		return StatusMsg{Text: fmt.Sprintf("Approved !%d", mrIID)}
	}
}

func (v *MRsView) mergeMR() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil || len(v.mrs) == 0 {
		return nil
	}
	if v.cursor >= len(v.mrs) {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	mrIID := v.mrs[v.cursor].IID
	return func() tea.Msg {
		if err := client.MergeMergeRequest(projectID, mrIID); err != nil {
			return StatusMsg{Text: fmt.Sprintf("Merge failed: %v", err), IsErr: true}
		}
		return StatusMsg{Text: fmt.Sprintf("Merged !%d", mrIID)}
	}
}

func (v *MRsView) openMRInBrowser() tea.Cmd {
	if v.cursor >= len(v.mrs) {
		return nil
	}
	cmd := openBrowserCmd(v.mrs[v.cursor].WebURL)
	if cmd == nil {
		return nil
	}
	return execBrowser(cmd)
}

// ============================================================================
// Helpers
// ============================================================================

func (v *MRsView) moveCursor(delta int) {
	n := len(v.mrs)
	if n == 0 {
		return
	}
	v.cursor += delta
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.cursor >= n {
		v.cursor = n - 1
	}
}

func (v *MRsView) clampCursor() {
	n := len(v.mrs)
	if v.cursor >= n {
		v.cursor = n - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}
