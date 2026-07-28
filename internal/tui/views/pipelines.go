package views

import (
	"fmt"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
	"github.com/Malvi1697/lazyglab/internal/util"
)

// Local key constants (the tui package's key table can't be imported here
// without a cycle, so the relevant subset is duplicated).
const (
	keyEnter      = "enter"
	keyEscape     = "esc"
	keyOpenBrowse = "o"
	keyRetry      = "R"
	keyCancel     = "C"
	keyRun        = "p" // run new pipeline / play manual job
	keyCopy       = "y" // copy the selected item's identifier, as in lazygit
	keyCopyLink   = "Y" // copy the selected item's URL
	keyComment    = "c" // write a comment, on a page that has a discussion
	keySystem     = "s" // show/hide GitLab's own record in a discussion
	keyTab        = "tab"
	keyShiftTab   = "shift+tab"
)

// PipelinesView is the self-contained cockpit view for pipelines, their jobs,
// and job logs.
type PipelinesView struct {
	ctx           *Context
	width, height int // last body size, tracked from tea.WindowSizeMsg / Body

	pipelines []gitlab.Pipeline
	cursor    int // indexes the visible (searched) list, not pipelines
	scroll    int // first visible row of the pipeline list, kept across frames

	search listSearch

	// jobs is the shared, interactive jobs panel — the same one the commit page
	// uses, so a pipeline is driven identically wherever it is shown.
	viewingJobs bool
	jobs        jobsPanel
}

// NewPipelinesView creates a PipelinesView bound to the shared session context.
func NewPipelinesView(ctx *Context) *PipelinesView {
	return &PipelinesView{ctx: ctx, jobs: jobsPanel{ctx: ctx}}
}

// Title implements View.
func (v *PipelinesView) Title() string { return "Pipelines" }

// Focus implements View: refreshes whatever is on screen, which is not always the
// pipeline list.
//
// Drilled into a pipeline's jobs, the jobs are what you are watching, so they are
// what gets refetched — reloading the list instead used to throw you back out of
// the panel every thirty seconds. An open log is left alone entirely: refetching
// it would yank the scroll back to the top of a file someone is reading.
func (v *PipelinesView) Focus() tea.Cmd {
	if v.viewingJobs {
		if v.jobs.showingTrace() {
			return nil
		}
		return v.jobs.load()
	}
	return v.load()
}

// ============================================================================
// Update
// ============================================================================

// Update implements View.
func (v *PipelinesView) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		return nil

	case PipelinesLoadedMsg:
		if msg.Err != nil {
			return statusCmd(fmt.Sprintf("Error loading pipelines: %v", msg.Err), true)
		}
		v.pipelines = msg.Pipelines
		v.clampCursor()
		return nil

	case JobsLoadedMsg:
		if msg.Err != nil {
			return statusCmd(fmt.Sprintf("Error loading jobs: %v", msg.Err), true)
		}
		v.jobs.setJobs(msg.Jobs)
		v.viewingJobs = true
		return nil

	case JobTraceLoadedMsg:
		if msg.Err != nil {
			return statusCmd(fmt.Sprintf("Error loading log: %v", msg.Err), true)
		}
		// A manual or pending job has nothing written yet; Enter on one used to look
		// like a key that did not work.
		if strings.TrimSpace(msg.Trace) == "" {
			return statusCmd("This job has not written a log yet", true)
		}
		v.jobs.setTrace(msg.Trace)
		return nil

	case JobActionDoneMsg:
		if msg.IsErr {
			return statusCmd(msg.Text, true)
		}
		return tea.Batch(v.jobs.load(), statusCmd(msg.Text, false))

	case PipelineActionDoneMsg:
		if msg.IsErr {
			return statusCmd(msg.Text, true)
		}
		return tea.Batch(v.load(), statusCmd(msg.Text, false))

	case tea.PasteMsg:
		if !v.viewingJobs {
			v.search.paste(msg.Content, &v.cursor)
		}
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}
	return nil
}

// CapturingText implements TextCapturer: while the search is being typed, the
// shell must not read the letters as its own commands.
func (v *PipelinesView) CapturingText() bool { return !v.viewingJobs && v.search.capturing() }

// visible is the pipelines matching the search; the cursor indexes it.
func (v *PipelinesView) visible() []gitlab.Pipeline {
	return filtered(v.pipelines, v.search.filter, func(p gitlab.Pipeline) string {
		return fmt.Sprintf("%d %s %s %s", p.ID, p.Status, p.Ref, p.CommitTitle)
	})
}

// selectedPipeline returns the highlighted pipeline, or nil.
func (v *PipelinesView) selectedPipeline() *gitlab.Pipeline {
	visible := v.visible()
	if v.cursor < 0 || v.cursor >= len(visible) {
		return nil
	}
	return &visible[v.cursor]
}

func (v *PipelinesView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	// The search owns the keys while it is open, so Esc clears it before it means
	// "back". Inside the jobs panel there is no list of ours to narrow.
	if !v.viewingJobs && v.search.handleKey(msg, &v.cursor) {
		return nil
	}

	// Esc: trace -> job list -> back to pipelines
	if key == keyEscape {
		switch {
		case v.viewingJobs && v.jobs.showingTrace():
			v.jobs.closeTrace()
		case v.viewingJobs:
			v.viewingJobs = false
			v.jobs.close()
		}
		return nil
	}

	if v.viewingJobs {
		if cmd, consumed := v.jobs.handleKey(key, v.height); consumed {
			return cmd
		}
		return nil
	}

	// Pipeline list navigation
	if act := components.NavFor(key); act != components.NavNone {
		v.cursor = components.ApplyNav(act, v.cursor, len(v.visible()), listRows(v.height))
		return nil
	}

	// Enter: load jobs for the selected pipeline
	if key == keyEnter {
		if p := v.selectedPipeline(); p != nil {
			return v.jobs.open(p.ID)
		}
		return nil
	}

	// Pipeline actions
	switch key {
	case keyRetry:
		if p := v.selectedPipeline(); p != nil {
			return confirmCmd(fmt.Sprintf("Retry pipeline #%d?", p.ID), v.retryPipeline())
		}
	case keyCancel:
		if p := v.selectedPipeline(); p != nil {
			return confirmCmd(fmt.Sprintf("Cancel pipeline #%d?", p.ID), v.cancelPipeline())
		}
	case keyCopy:
		if p := v.selectedPipeline(); p != nil {
			return copyRef(fmt.Sprintf("#%d", p.ID))
		}
	case keyCopyLink:
		if p := v.selectedPipeline(); p != nil {
			return copyLink(fmt.Sprintf("pipeline #%d", p.ID), p.WebURL)
		}
	case keyRun:
		ref := v.runRef()
		if ref == "" {
			return nil
		}
		return confirmCmd(fmt.Sprintf("Run new pipeline on %s?", ref), v.runPipeline())
	case keyOpenBrowse:
		return v.openPipelineInBrowser()
	}
	return nil
}

// ============================================================================
// Body / rendering
// ============================================================================

// Body implements View: a horizontal split with a list on the left and a
// detail/trace panel on the right.
func (v *PipelinesView) Body(width, height int) string {
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

	// Left list. Pipelines and jobs are separate lists, so each keeps its own
	// scroll position rather than inheriting the other's.
	// Drilled into jobs: the shared panel owns both halves.
	if v.viewingJobs {
		return v.jobs.body(width, height)
	}

	visible := v.visible()
	left := renderRowsBox(leftWidth, height,
		v.search.title("Pipelines", len(visible), len(v.pipelines)),
		len(visible), func(i int) string { return pipelineRow(visible[i]) },
		v.cursor, &v.scroll)

	detail := v.pipelineDetail()
	if detail == "" {
		detail = "Select an item to view details"
	}
	right := components.RenderPanel("Pipeline", strings.Split(detail, "\n"), rightWidth-4, height, false)

	return joinPanels(left, right, height)
}

// pipelineRow renders one pipeline list row (no ref column).
func pipelineRow(p gitlab.Pipeline) string {
	title := p.CommitTitle
	if title == "" {
		title = p.Ref
	}
	return fmt.Sprintf("%s %s %s",
		components.MutedStyle.Render(util.TimeAgoShort(p.CreatedAt)),
		components.StatusIconPadded(p.Status),
		styleCommitTitle(title),
	)
}

// jobItems returns display lines for jobs grouped by stage, plus a mapping from
// job index to display-row index (accounting for stage header lines). Header
// lines carry a leading "\x00" marker so renderListBox skips highlighting.
func (v *PipelinesView) pipelineDetail() string {
	if len(v.pipelines) == 0 {
		return "No pipelines"
	}
	selected := v.selectedPipeline()
	if selected == nil {
		if v.search.on() {
			return components.HelpDescStyle.Render("No pipeline matches " + v.search.filter.Query)
		}
		return ""
	}
	p := *selected

	var lines []string
	lines = append(lines,
		fmt.Sprintf("Status:  %s %s",
			components.StatusIcon(p.Status),
			lipgloss.NewStyle().Foreground(components.StatusColor(p.Status)).Render(p.Status),
		),
	)
	lines = append(lines, fmt.Sprintf("Ref:     %s", p.Ref))
	if p.CommitTitle != "" {
		lines = append(lines, fmt.Sprintf("Commit:  %s", p.CommitTitle))
	}
	if !p.CreatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Created: %s", util.TimeAgo(p.CreatedAt)))
	}
	if !p.UpdatedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("Updated: %s", util.TimeAgo(p.UpdatedAt)))
	}
	lines = append(lines, "")
	lines = append(lines, components.HelpDescStyle.Render(p.WebURL))
	return strings.Join(lines, "\n")
}

// ============================================================================
// KeyHints
// ============================================================================

// KeyHints implements View.
func (v *PipelinesView) KeyHints() []KeyHint {
	if v.viewingJobs {
		return v.jobs.keyHints()
	}
	return []KeyHint{
		{"Enter", "Jobs"},
		{"p", "Run"},
		{"R", "Retry"},
		{"C", "Cancel"},
		{"o", "Open"},
		{"y/Y", "Copy #/link"},
		v.search.hint(),
	}
}

// ============================================================================
// Commands (async API calls)
// ============================================================================

func (v *PipelinesView) load() tea.Cmd {
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

func (v *PipelinesView) retryPipeline() tea.Cmd {
	p := v.selectedPipeline()
	if p == nil || v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	pipelineID := p.ID
	return func() tea.Msg {
		if err := client.RetryPipeline(projectID, pipelineID); err != nil {
			return PipelineActionDoneMsg{Text: fmt.Sprintf("Retry failed: %v", err), IsErr: true}
		}
		return PipelineActionDoneMsg{Text: fmt.Sprintf("Retried pipeline #%d", pipelineID)}
	}
}

func (v *PipelinesView) cancelPipeline() tea.Cmd {
	p := v.selectedPipeline()
	if p == nil || v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	pipelineID := p.ID
	return func() tea.Msg {
		if err := client.CancelPipeline(projectID, pipelineID); err != nil {
			return PipelineActionDoneMsg{Text: fmt.Sprintf("Cancel failed: %v", err), IsErr: true}
		}
		return PipelineActionDoneMsg{Text: fmt.Sprintf("Canceled pipeline #%d", pipelineID)}
	}
}

// runRef returns the ref a new pipeline would run on (selected branch or the
// project default), or "" when unavailable.
func (v *PipelinesView) runRef() string {
	if v.ctx == nil || v.ctx.Project == nil {
		return ""
	}
	if v.ctx.Branch != nil {
		return v.ctx.Branch.Name
	}
	return v.ctx.Project.DefaultBranch
}

func (v *PipelinesView) runPipeline() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil {
		return nil
	}
	client := v.ctx.Client
	projectID := v.ctx.Project.ID
	ref := v.runRef()
	return func() tea.Msg {
		p, err := client.RunPipeline(projectID, ref)
		if err != nil {
			return PipelineActionDoneMsg{Text: fmt.Sprintf("Run failed: %v", err), IsErr: true}
		}
		return PipelineActionDoneMsg{Text: fmt.Sprintf("Pipeline #%d started on %s", p.ID, ref)}
	}
}

func (v *PipelinesView) openPipelineInBrowser() tea.Cmd {
	p := v.selectedPipeline()
	if p == nil {
		return nil
	}
	cmd := openBrowserCmd(p.WebURL)
	if cmd == nil {
		return nil
	}
	return execBrowser(cmd)
}

// ============================================================================
// Helpers
// ============================================================================

func (v *PipelinesView) clampCursor() {
	v.cursor = clampCursor(v.cursor, len(v.visible()))
}

// confirmCmd returns a command that asks the shell to confirm a destructive
// action before running it.
func confirmCmd(prompt string, action tea.Cmd) tea.Cmd {
	return func() tea.Msg {
		return ConfirmMsg{Prompt: prompt, Action: action}
	}
}

// execBrowser runs an openBrowserCmd and reports failures via StatusMsg.
func execBrowser(cmd *exec.Cmd) tea.Cmd {
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return StatusMsg{Text: fmt.Sprintf("Failed to open browser: %v", err), IsErr: true}
		}
		return nil
	})
}

// listRows is how many list rows a view of the given height shows, i.e. the box
// height minus its borders. It is the page size for navigation.
func listRows(height int) int {
	rows := height - 2
	if rows < 1 {
		rows = 1
	}
	return rows
}
