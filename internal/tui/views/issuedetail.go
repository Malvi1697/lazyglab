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

// issueDetail is the full-screen issue page: what it says, who it is on, and the
// conversation about it.
type issueDetail struct {
	ctx *Context

	active bool
	issue  *gitlab.Issue

	notesBox
	pageFrame

	descScrollable bool

	iid       int // the issue on screen; replies for others are stale
	requested int
	loading   bool
	scroll    int

	focus pageFocus

	pages map[int][]gitlab.Note
	order []int
}

// issuePagesKept bounds the cache of fetched discussions.
const issuePagesKept = 8

func newIssueDetail(ctx *Context) issueDetail {
	return issueDetail{ctx: ctx, pages: map[int][]gitlab.Note{}}
}

// openAt drills into an issue; Enter means "show me this one".
func (d *issueDetail) openAt(issue *gitlab.Issue, index, total int) tea.Cmd {
	return d.showAt(issue, index, total, 0)
}

// stepAt is openAt for a step to a neighbour, which waits for the key to settle.
func (d *issueDetail) stepAt(issue *gitlab.Issue, index, total int) tea.Cmd {
	return d.showAt(issue, index, total, stepSettleDelay)
}

// issueFetchMsg asks the page to fetch a discussion, if it is still the one shown.
type issueFetchMsg struct{ iid int }

func (d *issueDetail) showAt(issue *gitlab.Issue, index, total int, delay time.Duration) tea.Cmd {
	if issue == nil {
		return nil
	}
	d.active = true
	d.issue = issue
	d.placeIn(index, total)
	d.focus = focusPage
	d.resetNotes()
	d.scroll = 0
	d.iid, d.requested = issue.IID, 0

	if notes, ok := d.pages[issue.IID]; ok {
		d.loading = false
		d.setNotes(notes)
		return nil
	}

	d.loading = true
	if delay <= 0 {
		return d.load(issue.IID)
	}
	return tea.Tick(delay, func(time.Time) tea.Msg { return issueFetchMsg{iid: issue.IID} })
}

func (d *issueDetail) close() {
	d.active = false
	d.issue = nil
	d.focus = focusPage
	d.resetNotes()
	d.iid, d.requested, d.scroll = 0, 0, 0
}

// reload refetches the open issue's discussion: a new comment is the one thing about an
// issue that arrives while you are looking at it.
func (d *issueDetail) reload() tea.Cmd {
	if !d.active || d.iid == 0 {
		return nil
	}
	delete(d.pages, d.iid)
	return d.load(d.iid)
}

// readingBody reports whether the thread has the screen, so the host knows the arrows
// are not its to act on.
func (d *issueDetail) readingBody() bool { return d.threadOpen }

// load fetches the issue's discussion — the one thing the list row does not carry.
func (d *issueDetail) load(iid int) tea.Cmd {
	if d.ctx == nil || d.ctx.Project == nil || d.ctx.Client == nil {
		return nil
	}
	client, projectID := d.ctx.Client, d.ctx.Project.ID
	d.requested = iid
	d.loading = true
	return func() tea.Msg {
		notes, err := client.ListIssueNotes(projectID, iid)
		return IssueNotesLoadedMsg{IID: iid, Notes: notes, Err: err}
	}
}

func (d *issueDetail) update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case issueFetchMsg:
		if !d.active || m.iid != d.iid || d.requested == d.iid || d.issue == nil {
			return nil
		}
		if _, cached := d.pages[d.iid]; cached {
			return nil
		}
		return d.load(d.iid)

	case IssueNotesLoadedMsg:
		if m.IID != d.iid {
			return nil // a stale reply
		}
		d.loading = false
		if m.Err != nil {
			return statusCmd(fmt.Sprintf("Error loading the discussion: %v", m.Err), true)
		}
		d.setNotes(m.Notes)
		d.remember(m.IID, m.Notes)
		return nil

	case commentWrittenMsg:
		if m.err != nil {
			return statusCmd(fmt.Sprintf("Could not open an editor: %v", m.err), true)
		}
		if m.body == "" {
			return statusCmd("Empty comment, nothing posted", false)
		}
		return d.postComment(m.body)

	case IssueActionDoneMsg:
		if m.IsErr {
			return statusCmd(m.Text, true)
		}
		// A new comment changes the discussion, so its cached copy is void.
		delete(d.pages, d.iid)
		return tea.Batch(d.load(d.iid), statusCmd(m.Text, false))
	}
	return nil
}

func (d *issueDetail) remember(iid int, notes []gitlab.Note) {
	if iid == 0 {
		return
	}
	if d.pages == nil {
		d.pages = map[int][]gitlab.Note{}
	}
	if _, seen := d.pages[iid]; !seen {
		d.order = append(d.order, iid)
		for len(d.order) > issuePagesKept {
			delete(d.pages, d.order[0])
			d.order = d.order[1:]
		}
	}
	d.pages[iid] = notes
}

func (d *issueDetail) handleKey(key string, height int) tea.Cmd {
	if d.threadOpen {
		switch key {
		case keyEscape:
			d.closeThread()
			return nil
		case keyComment:
			return d.comment()
		case keySystem:
			return d.toggleSystem()
		}
		d.threadKey(key, height)
		return nil
	}

	switch key {
	case keyTab:
		d.focus = cycleFocus(d.focus, 1, false, false, true)
		return nil
	case keyShiftTab:
		d.focus = cycleFocus(d.focus, -1, false, false, true)
		return nil
	}

	if d.focus == focusNotes {
		switch key {
		case keyEscape:
			d.focus = focusPage
			return nil
		case keyComment:
			return d.comment()
		case keySystem:
			return d.toggleSystem()
		}
		if d.notesKey(key, height) {
			return nil
		}
		return d.copyKeys(key)
	}

	if act := components.NavFor(key); act != components.NavNone {
		d.scroll = components.ApplyNav(act, d.scroll, len(d.topLines(0)), listRows(height))
		return nil
	}

	switch key {
	case keyEscape:
		d.close()
		return nil
	case keyEnter:
		// The discussion is what a page adds to a list row, so step into it.
		d.focus = focusNotes
		return nil
	case keyComment:
		return d.comment()
	case keyCopy, keyCopyLink:
		return d.copyKeys(key)
	case keyOpenBrowse:
		if d.issue == nil {
			return nil
		}
		if cmd := openBrowserCmd(d.issue.WebURL); cmd != nil {
			return execBrowser(cmd)
		}
	}
	return nil
}

func (d *issueDetail) copyKeys(key string) tea.Cmd {
	if d.issue == nil {
		return nil
	}
	ref := fmt.Sprintf("#%d", d.issue.IID)
	switch key {
	case keyCopy:
		return copyRef(ref)
	case keyCopyLink:
		return copyLink(ref, d.issue.WebURL)
	}
	return nil
}

func (d *issueDetail) comment() tea.Cmd {
	if d.issue == nil {
		return nil
	}
	return composeComment(fmt.Sprintf("#%d %s", d.issue.IID, d.issue.Title))
}

func (d *issueDetail) postComment(body string) tea.Cmd {
	if d.issue == nil || d.ctx == nil || d.ctx.Project == nil || d.ctx.Client == nil {
		return nil
	}
	client, projectID, iid := d.ctx.Client, d.ctx.Project.ID, d.issue.IID
	return func() tea.Msg {
		if err := client.CreateIssueNote(projectID, iid, body); err != nil {
			return IssueActionDoneMsg{Text: fmt.Sprintf("Comment failed: %v", err), IsErr: true}
		}
		return IssueActionDoneMsg{Text: fmt.Sprintf("Commented on #%d", iid)}
	}
}

func (d *issueDetail) body(width, height int) string {
	if d.threadOpen {
		return components.RenderPanel(d.threadTitle(),
			splitLines(d.threadView(width, height)), width, height, true)
	}

	pageWidth := width - 2*arrowGutter
	if pageWidth < 20 {
		return d.page(width, height)
	}
	return d.withArrows(d.page(pageWidth, height), d.hasPrev(), d.hasNext())
}

// page renders the issue above its discussion.
func (d *issueDetail) page(width, height int) string {
	d.pageWidth = width

	top := d.topLines(width - 2)
	topHeight := len(top) + 1
	if maxTop := height * 3 / 5; topHeight > maxTop {
		topHeight = maxTop
	}
	if topHeight < 3 {
		topHeight = 3
	}

	const gap = 1
	bottomHeight := height - topHeight - gap
	if bottomHeight < 4 {
		d.descScrollable = len(top) > height-1
		return components.RenderPanel(d.title(len(top), height-1), d.window(top, height-1),
			width, height, true)
	}
	d.descScrollable = len(top) > topHeight-1

	return lipgloss.JoinVertical(lipgloss.Left,
		components.RenderPanel(d.title(len(top), topHeight-1), d.window(top, topHeight-1),
			width, topHeight, d.focus == focusPage),
		"",
		d.notesPanel(width, bottomHeight, d.focus == focusNotes, d.loading),
	)
}

func (d *issueDetail) title(lines, rows int) string {
	out := "Issue"
	if d.issue != nil {
		out = fmt.Sprintf("#%d", d.issue.IID)
	}
	out += d.counter()
	if lines > rows {
		end := min(d.scroll+rows, lines)
		out = fmt.Sprintf("%s  ·  %d-%d of %d", out, d.scroll+1, end, lines)
	}
	return out
}

func (d *issueDetail) window(lines []string, rows int) []string {
	if rows < 1 {
		rows = 1
	}
	if d.scroll > len(lines)-rows {
		d.scroll = max(0, len(lines)-rows)
	}
	if d.scroll < 0 {
		d.scroll = 0
	}
	return lines[d.scroll:min(d.scroll+rows, len(lines))]
}

// topLines is the title, who it is on, and what it says.
func (d *issueDetail) topLines(width int) []string {
	issue := d.issue
	if issue == nil {
		return []string{"No issue"}
	}

	var out []string
	add := func(s string) { out = append(out, s) }
	field := func(label, value string) {
		if value != "" {
			add(components.HelpDescStyle.Render(label) + value)
		}
	}
	wrap := func(s string) {
		if width <= 0 {
			add(s)
			return
		}
		out = append(out, components.WrapLine(s, width)...)
	}

	wrap(components.TitleStyle.Render(issue.Title))
	add("")
	add(components.HelpDescStyle.Render("state   ") + issueState(issue.State))
	field("author  ", issue.Author)
	if len(issue.Assignees) > 0 {
		field("assign  ", strings.Join(issue.Assignees, ", "))
	} else {
		field("assign  ", components.HelpDescStyle.Render("nobody"))
	}
	if len(issue.Labels) > 0 {
		field("labels  ", strings.Join(issue.Labels, ", "))
	}
	if !issue.CreatedAt.IsZero() {
		field("opened  ", util.TimeAgo(issue.CreatedAt))
	}
	if !issue.UpdatedAt.IsZero() {
		field("updated ", util.TimeAgo(issue.UpdatedAt))
	}

	if body := strings.TrimSpace(issue.Description); body != "" {
		add("")
		for _, line := range strings.Split(body, "\n") {
			wrap(line)
		}
	} else {
		add("")
		wrap(components.HelpDescStyle.Render("No description."))
	}

	add("")
	wrap(components.HelpDescStyle.Render(issue.WebURL))
	return out
}

// issueState colours the state: closed is done, open is work.
func issueState(state string) string {
	if state == "opened" {
		return lipgloss.NewStyle().Foreground(components.ColorSuccess).Render("open")
	}
	return components.MutedStyle.Render(state)
}

func (d *issueDetail) keyHints() []KeyHint {
	if d.threadOpen {
		return d.threadHints()
	}
	if d.focus == focusNotes {
		return append(d.boxHints(),
			KeyHint{"←/→ h/l", "Issue"},
			KeyHint{"Esc", "Back"},
		)
	}

	hints := []KeyHint{
		{"←/→ h/l", "Prev/next issue"},
		{"Enter", "Discussion"},
	}
	if d.descScrollable {
		hints = append(hints, KeyHint{"j/k", "Scroll"})
	}
	return append(hints,
		KeyHint{"c", "Comment"},
		KeyHint{"y/Y", "Copy #/link"},
		KeyHint{"o", "Open"},
		KeyHint{"Esc", "Back"},
	)
}
