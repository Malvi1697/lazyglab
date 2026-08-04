package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// Local key constant specific to the Issues view (see pipelines.go for the shared
// subset).
const keyClose = "c"

// IssuesView is the self-contained cockpit view for issues.
type IssuesView struct {
	ctx           *Context
	width, height int // last body size, tracked from tea.WindowSizeMsg / Body

	rowList[gitlab.Issue]

	// detail is the in-place issue page, opened with Enter: the issue and its discussion,
	// which is the one thing the list row does not carry.
	detail issueDetail
}

// NewIssuesView creates an IssuesView bound to the shared session context.
func NewIssuesView(ctx *Context) *IssuesView {
	v := &IssuesView{ctx: ctx, detail: newIssueDetail(ctx)}
	v.match = func(i gitlab.Issue) string {
		return fmt.Sprintf("#%d %s %s %s", i.IID, i.Title, i.Author, strings.Join(i.Labels, " "))
	}
	return v
}

// Title implements View.
func (v *IssuesView) Title() string { return "Issues" }

// Focus implements View: refreshes what is on screen — an open issue's discussion,
// where a new comment is exactly the thing worth noticing, or the list.
func (v *IssuesView) Focus() tea.Cmd {
	if v.detail.active {
		if v.detail.readingBody() {
			return nil
		}
		return v.detail.reload()
	}
	return v.load()
}

// Update implements View.
func (v *IssuesView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return nil

	case IssuesLoadedMsg:
		if msg.Err != nil {
			return statusCmd(fmt.Sprintf("Error loading issues: %v", msg.Err), true)
		}
		v.setItems(msg.Issues)
		return nil

	case tea.PasteMsg:
		if !v.detail.active {
			v.paste(msg.Content)
		}
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}

	// Everything else may belong to the issue page.
	return v.detail.update(msg)
}

// CapturingText implements TextCapturer: while the search is being typed, the shell
// must not read the letters as its own commands.
func (v *IssuesView) CapturingText() bool { return !v.detail.active && v.capturing() }

// stepIssue moves to the neighbouring issue, keeping the page open.
func (v *IssuesView) stepIssue(step int) tea.Cmd {
	visible := v.visible()
	next := v.cursor + step
	if next < 0 || next >= len(visible) {
		return nil // already at an end; the arrow is drawn faint there
	}
	v.cursor = next
	return v.detail.stepAt(v.selected(), v.cursor, len(visible))
}

func (v *IssuesView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	if v.detail.active {
		// Stepping to the neighbouring issue belongs to the list's owner — but not while the
		// thread is open, where the arrows belong to what you are reading.
		if step, ok := stepKey(key); ok && !v.detail.readingBody() {
			return v.stepIssue(step)
		}
		return v.detail.handleKey(key, v.height)
	}

	if v.navigate(msg, v.height) {
		return nil
	}

	issue := v.selected()
	if issue == nil {
		return nil
	}
	switch key {
	case keyEnter:
		return v.detail.openAt(issue, v.cursor, len(v.visible()))
	case keyClose:
		action := "Close"
		if issue.State != "opened" {
			action = "Reopen"
		}
		return confirmCmd(fmt.Sprintf("%s #%d %s?", action, issue.IID, issue.Title), v.toggleIssue())
	case keyCopy:
		return copyRef(fmt.Sprintf("#%d", issue.IID))
	case keyCopyLink:
		return copyLink(fmt.Sprintf("#%d", issue.IID), issue.WebURL)
	case keyOpenBrowse:
		if cmd := openBrowserCmd(issue.WebURL); cmd != nil {
			return execBrowser(cmd)
		}
	}
	return nil
}

// Body implements View: the issues, full width, in the columns every other list uses.
func (v *IssuesView) Body(width, height int) string {
	v.width = width
	v.height = height

	// Drilled into an issue: the page takes the whole body.
	if v.detail.active {
		return v.detail.body(width, height)
	}

	return v.box(width, height, "Issues", issueRow, true)
}

// issueRow describes one issue row.
func issueRow(issue gitlab.Issue) listRow {
	kind, subject := splitConventional(issue.Title)
	return listRow{
		ref:     fmt.Sprintf("#%d", issue.IID),
		kind:    kind,
		subject: subject,
		author:  issue.Author,
		extra:   strings.Join(issue.Assignees, ", "),
		stamp:   commitStamp(issue.UpdatedAt),
	}
}

// KeyHints implements View.
func (v *IssuesView) KeyHints() []KeyHint {
	if v.detail.active {
		return v.detail.keyHints()
	}
	return []KeyHint{
		{"Enter", "Issue page"},
		{"c", "Close/reopen"},
		{"o", "Open"},
		{"y/Y", "Copy #/link"},
		v.search.hint(),
	}
}

func (v *IssuesView) load() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	return func() tea.Msg {
		issues, err := client.ListIssues(projectID)
		return IssuesLoadedMsg{Issues: issues, Err: err}
	}
}

func (v *IssuesView) toggleIssue() tea.Cmd {
	selected := v.selected()
	if selected == nil || v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	issue := *selected
	return func() tea.Msg {
		var err error
		if issue.State == "opened" {
			err = client.CloseIssue(projectID, issue.IID)
		} else {
			err = client.ReopenIssue(projectID, issue.IID)
		}
		if err != nil {
			return StatusMsg{Text: fmt.Sprintf("Toggle issue failed: %v", err), IsErr: true}
		}
		action := "Closed"
		if issue.State != "opened" {
			action = "Reopened"
		}
		return StatusMsg{Text: fmt.Sprintf("%s #%d", action, issue.IID)}
	}
}
