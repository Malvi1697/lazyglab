package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/util"
)

// Local key constants specific to the Todos view (see pipelines.go for the
// shared subset).
const (
	keyDone    = "d"
	keyDoneAll = "D"
)

// TodosView is the user's GitLab To-Do list: reviews asked of them, mentions,
// assignments and failed pipelines they own.
//
// It is the one view that ignores the selected project. Every other view answers
// "what is happening here"; this one answers "what is waiting on me", which is
// the question you have before you have picked a project at all.
type TodosView struct {
	ctx           *Context
	width, height int // last body size, tracked from tea.WindowSizeMsg / Body

	todos  []gitlab.Todo
	loaded bool // a reply has arrived, so an empty list really is empty
	cursor int  // indexes the visible (searched) list, not todos
	scroll int  // first visible row, kept across frames

	search listSearch

	status string
}

// NewTodosView creates a TodosView bound to the shared session context.
func NewTodosView(ctx *Context) *TodosView { return &TodosView{ctx: ctx} }

// Title implements View.
func (v *TodosView) Title() string { return "Todos" }

// Focus implements View: loads the to-do list. No project needed.
func (v *TodosView) Focus() tea.Cmd { return v.load() }

// ============================================================================
// Update
// ============================================================================

// Update implements View.
func (v *TodosView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return nil

	case TodosLoadedMsg:
		if msg.Err != nil {
			v.status = fmt.Sprintf("Error loading todos: %v", msg.Err)
			return nil
		}
		v.loaded = true
		v.todos = msg.Todos
		v.clampCursor()
		return nil

	case TodoActionDoneMsg:
		// Clearing a to-do removes it from the list, so the list has to be refetched
		// for the screen to match what GitLab now thinks.
		if msg.IsErr {
			return statusCmd(msg.Text, true)
		}
		return tea.Batch(v.load(), statusCmd(msg.Text, false))

	case StatusMsg:
		v.status = msg.Text
		return nil

	case tea.PasteMsg:
		v.search.paste(msg.Content, &v.cursor)
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}
	return nil
}

// CapturingText implements TextCapturer: while the search is being typed, the
// shell must not read the letters as its own commands.
func (v *TodosView) CapturingText() bool { return v.search.capturing() }

// visible is the to-dos matching the search; the cursor indexes it.
func (v *TodosView) visible() []gitlab.Todo {
	return filtered(v.todos, v.search.filter, func(t gitlab.Todo) string {
		return strings.Join([]string{t.Reference, t.Title, t.ProjectPath, t.Author,
			t.Action, todoActionWord(t.Action)}, " ")
	})
}

// selected returns the highlighted to-do, or nil.
func (v *TodosView) selected() *gitlab.Todo {
	visible := v.visible()
	if v.cursor < 0 || v.cursor >= len(visible) {
		return nil
	}
	return &visible[v.cursor]
}

func (v *TodosView) handleKey(msg tea.KeyMsg) tea.Cmd {
	if v.search.handleKey(msg, &v.cursor) {
		return nil
	}

	key := msg.String()
	if act := components.NavFor(key); act != components.NavNone {
		v.cursor = components.ApplyNav(act, v.cursor, len(v.visible()), listRows(v.height))
		return nil
	}

	// Clearing the whole list works with an empty cursor, so it comes first.
	if key == keyDoneAll {
		if len(v.todos) == 0 {
			return nil
		}
		return confirmCmd(fmt.Sprintf("Mark all %d todos as done?", len(v.todos)), v.markAllDone())
	}

	todo := v.selected()
	if todo == nil {
		return nil
	}
	switch key {
	case keyDone:
		return confirmCmd(fmt.Sprintf("Mark done: %s?", todoLabel(*todo)), v.markDone(*todo))
	case keyEnter, keyOpenBrowse:
		// A to-do is a pointer at something on the web: an MR to review, an issue you
		// were named in. There is no deeper screen for it inside lazyglab, so Enter
		// means the same as o rather than nothing at all.
		if cmd := openBrowserCmd(todo.WebURL); cmd != nil {
			return execBrowser(cmd)
		}
	}
	return nil
}

// ============================================================================
// Body / rendering
// ============================================================================

// Body implements View: the list on the left, the highlighted to-do on the right.
func (v *TodosView) Body(width, height int) string {
	v.width = width
	v.height = height

	leftWidth := width * 55 / 100
	if leftWidth < 20 {
		leftWidth = 20
	}
	if leftWidth > width {
		leftWidth = width
	}
	rightWidth := width - leftWidth

	visible := v.visible()
	left := renderListBox(leftWidth, height,
		v.search.title("Todos", len(visible), len(v.todos)),
		v.todoItems(visible), v.cursor, &v.scroll)

	right := components.RenderPanel(v.detailTitle(), splitLines(v.todoDetail()),
		rightWidth-4, height, false)

	return joinPanels(left, right, height)
}

func (v *TodosView) detailTitle() string {
	if t := v.selected(); t != nil && t.Reference != "" {
		return "Todo (" + t.Reference + ")"
	}
	return "Todo"
}

// todoItems renders the list rows: when it arrived, why it is there, which
// project, and what it is about.
func (v *TodosView) todoItems(todos []gitlab.Todo) []string {
	items := make([]string, len(todos))
	for i, t := range todos {
		project := t.ProjectPath
		if i := strings.LastIndex(project, "/"); i >= 0 {
			project = project[i+1:] // the group repeats on every row; the project does not
		}
		items[i] = fmt.Sprintf("%s %s %s  %s",
			components.MutedStyle.Render(util.TimeAgoShort(t.CreatedAt)),
			todoActionTag(t.Action),
			components.MutedStyle.Render(components.PadRight(components.Truncate(project, 16), 16)),
			todoLabel(t),
		)
	}
	return items
}

func (v *TodosView) todoDetail() string {
	if len(v.todos) == 0 {
		// Before the first reply an empty list means "not yet", not "nothing" — and
		// telling someone their plate is clear when it is not is the worse mistake.
		if !v.loaded {
			return components.HelpDescStyle.Render("Loading…")
		}
		return "Nothing is waiting on you"
	}
	t := v.selected()
	if t == nil {
		if v.search.on() {
			return components.HelpDescStyle.Render("No todo matches " + v.search.filter.Query)
		}
		return ""
	}

	var lines []string
	add := func(label, value string) {
		if value != "" {
			lines = append(lines, components.HelpDescStyle.Render(label)+value)
		}
	}

	lines = append(lines, components.TitleStyle.Render(todoLabel(*t)))
	lines = append(lines, "")
	add("why      ", todoActionWord(t.Action))
	add("project  ", t.ProjectPath)
	add("from     ", t.Author)
	add("kind     ", t.Target)
	add("state    ", t.TargetState)
	if !t.CreatedAt.IsZero() {
		add("since    ", util.TimeAgo(t.CreatedAt))
	}
	if t.Body != "" && t.Body != t.Title {
		lines = append(lines, "")
		lines = append(lines, components.WrapLine(t.Body, maxInt(v.width/2-8, 20))...)
	}
	lines = append(lines, "", components.HelpDescStyle.Render(t.WebURL))
	return strings.Join(lines, "\n")
}

// todoLabel is the to-do's reference and title as one string.
func todoLabel(t gitlab.Todo) string {
	if t.Reference == "" {
		return t.Title
	}
	return t.Reference + " " + t.Title
}

// todoActionTag is the fixed-width column saying why a to-do exists. Only the
// ones that mean something is broken or blocked take a colour; the rest are the
// same weight as the other metadata, or the list would be a wall of colour.
func todoActionTag(action string) string {
	word := components.PadRight(components.Truncate(todoActionShort(action), 8), 8)
	switch action {
	case "build_failed", "unmergeable", "merge_train_removed":
		return lipgloss.NewStyle().Foreground(components.ColorError).Render(word)
	case "approval_required", "review_requested":
		return lipgloss.NewStyle().Foreground(components.ColorWarning).Render(word)
	default:
		return components.MutedStyle.Render(word)
	}
}

// todoActionShort is the one-word form of GitLab's action name, for the column.
func todoActionShort(action string) string {
	switch action {
	case "assigned":
		return "assigned"
	case "review_requested":
		return "review"
	case "approval_required":
		return "approve"
	case "mentioned", "directly_addressed":
		return "mention"
	case "build_failed":
		return "ci"
	case "marked":
		return "marked"
	case "unmergeable":
		return "conflict"
	case "member_access_requested", "okr_checkin_requested":
		return "request"
	case "merge_train_removed":
		return "train"
	case "review_submitted":
		return "reviewed"
	default:
		return strings.ReplaceAll(action, "_", " ")
	}
}

// todoActionWord spells out why the to-do is there, for the detail panel.
func todoActionWord(action string) string {
	switch action {
	case "assigned":
		return "assigned to you"
	case "review_requested":
		return "your review was requested"
	case "approval_required":
		return "your approval is required"
	case "mentioned":
		return "you were mentioned"
	case "directly_addressed":
		return "you were addressed directly"
	case "build_failed":
		return "the pipeline failed"
	case "marked":
		return "you added it yourself"
	case "unmergeable":
		return "it cannot be merged"
	case "member_access_requested":
		return "someone asked for access"
	case "merge_train_removed":
		return "it was dropped from the merge train"
	case "review_submitted":
		return "a review was submitted"
	default:
		return strings.ReplaceAll(action, "_", " ")
	}
}

// ============================================================================
// KeyHints
// ============================================================================

// KeyHints implements View.
func (v *TodosView) KeyHints() []KeyHint {
	return []KeyHint{
		{"Enter/o", "Open"},
		{"d", "Done"},
		{"D", "All done"},
		v.search.hint(),
	}
}

// ============================================================================
// Commands (async API calls)
// ============================================================================

func (v *TodosView) load() tea.Cmd {
	if v.ctx == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	return func() tea.Msg {
		todos, err := client.ListTodos()
		return TodosLoadedMsg{Todos: todos, Err: err}
	}
}

func (v *TodosView) markDone(t gitlab.Todo) tea.Cmd {
	if v.ctx == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	return func() tea.Msg {
		if err := client.MarkTodoDone(t.ID); err != nil {
			return TodoActionDoneMsg{Text: fmt.Sprintf("Could not mark it done: %v", err), IsErr: true}
		}
		return TodoActionDoneMsg{Text: "Done: " + todoLabel(t)}
	}
}

func (v *TodosView) markAllDone() tea.Cmd {
	if v.ctx == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	count := len(v.todos)
	return func() tea.Msg {
		if err := client.MarkAllTodosDone(); err != nil {
			return TodoActionDoneMsg{Text: fmt.Sprintf("Could not clear the list: %v", err), IsErr: true}
		}
		return TodoActionDoneMsg{Text: fmt.Sprintf("Cleared %d todos", count)}
	}
}

// ============================================================================
// Helpers
// ============================================================================

func (v *TodosView) clampCursor() {
	v.cursor = clampCursor(v.cursor, len(v.visible()))
}
