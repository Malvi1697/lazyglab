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
	cursor  int
	scroll  int // first visible row, kept across frames

	status string
}

// NewCommitsView creates a CommitsView bound to the shared session context.
func NewCommitsView(ctx *Context) *CommitsView { return &CommitsView{ctx: ctx} }

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
			v.status = fmt.Sprintf("Error loading commits: %v", msg.Err)
			return nil
		}
		v.commits = msg.Commits
		v.clampCursor()
		v.status = fmt.Sprintf("Loaded %d commits", len(msg.Commits))
		return nil

	case StatusMsg:
		v.status = msg.Text
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}
	return nil
}

func (v *CommitsView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if act := components.NavFor(key); act != components.NavNone {
		v.cursor = components.ApplyNav(act, v.cursor, len(v.commits), listRows(v.height))
		return nil
	}

	if key == keyOpenBrowse {
		return v.openCommitInBrowser()
	}
	if key == keyEnter {
		return v.showPipeline()
	}
	return nil
}

// showPipeline asks the shell to open the selected commit's pipeline.
func (v *CommitsView) showPipeline() tea.Cmd {
	if v.cursor >= len(v.commits) {
		return nil
	}
	sha := v.commits[v.cursor].ShortID
	return func() tea.Msg { return ShowCommitPipelineMsg{ShortSHA: sha} }
}

// ============================================================================
// Body / rendering
// ============================================================================

// Body implements View: a horizontal split with a list on the left and a
// detail panel on the right.
func (v *CommitsView) Body(width, height int) string {
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

	left := renderListBox(leftWidth, height, "Commits", v.commitItems(), v.cursor, &v.scroll)

	detail := v.commitDetail()
	if detail == "" {
		detail = "Select an item to view details"
	}
	right := components.RenderBox("Commit", splitLines(detail), rightWidth, height, components.ColorSecondary, components.ColorPrimary)

	return joinH(left, right)
}

// commitItems renders the commit list rows.
func (v *CommitsView) commitItems() []string {
	items := make([]string, len(v.commits))
	for i, c := range v.commits {
		icon := "  "
		if c.Status != "" {
			icon = components.StatusIconPadded(c.Status)
		}
		items[i] = fmt.Sprintf("%s %s %s  %s",
			util.TimeAgoShort(c.CreatedAt),
			icon,
			components.PadRight(c.ShortID, 8),
			c.Title,
		)
	}
	return items
}

func (v *CommitsView) commitDetail() string {
	if len(v.commits) == 0 {
		return "No commits"
	}
	if v.cursor >= len(v.commits) {
		return ""
	}
	c := v.commits[v.cursor]

	return fmt.Sprintf("%s\n\n%s\n\nAuthor: %s\n%s\n\n%s",
		components.TitleStyle.Render(c.ShortID),
		c.Title,
		c.AuthorName,
		util.TimeAgo(c.CreatedAt),
		components.HelpDescStyle.Render(c.WebURL),
	)
}

// ============================================================================
// KeyHints
// ============================================================================

// KeyHints implements View.
func (v *CommitsView) KeyHints() []KeyHint {
	return []KeyHint{
		{"o", "Open"},
	}
}

// ============================================================================
// Commands (async API calls)
// ============================================================================

func (v *CommitsView) load() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	ref := ""
	if v.ctx.Branch != nil {
		ref = v.ctx.Branch.Name
	}
	return func() tea.Msg {
		commits, err := client.ListCommits(projectID, ref)
		return CommitsLoadedMsg{Commits: commits, Err: err}
	}
}

func (v *CommitsView) openCommitInBrowser() tea.Cmd {
	if v.cursor >= len(v.commits) {
		return nil
	}
	cmd := openBrowserCmd(v.commits[v.cursor].WebURL)
	if cmd == nil {
		return nil
	}
	return execBrowser(cmd)
}

// ============================================================================
// Helpers
// ============================================================================

func (v *CommitsView) clampCursor() {
	n := len(v.commits)
	if v.cursor >= n {
		v.cursor = n - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
}
