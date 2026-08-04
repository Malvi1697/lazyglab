package views

import (
	"fmt"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
	"github.com/Malvi1697/lazyglab/internal/tui/components"
)

// Local key constants (the tui package's key table can't be imported here without a
// cycle, so the relevant subset is duplicated).
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
	keyToggleBox  = "t" // fold the second box away, for a window with few rows
	keyTab        = "tab"
	keyShiftTab   = "shift+tab"
)

// PipelinesView is the self-contained cockpit view for pipelines, their jobs, and job
// logs.
type PipelinesView struct {
	ctx           *Context
	width, height int // last body size, tracked from tea.WindowSizeMsg / Body

	rowList[gitlab.Pipeline]

	// stages is how far each pipeline got, by pipeline ID: the row of marks GitLab shows
	// in its own list.
	stages map[int][]gitlab.Stage
	// stagesBox names the stages behind the marks, and starts folded.
	stagesBox foldBox

	// jobs is the shared, interactive jobs panel — the same one the commit page uses, so a
	// pipeline is driven identically wherever it is shown.
	viewingJobs bool
	jobs        jobsPanel
}

// NewPipelinesView creates a PipelinesView bound to the shared session context.
func NewPipelinesView(ctx *Context) *PipelinesView {
	v := &PipelinesView{ctx: ctx, jobs: jobsPanel{ctx: ctx},
		stagesBox: foldBox{name: "stages", folded: true}}
	v.match = func(p gitlab.Pipeline) string {
		return fmt.Sprintf("%d %s %s %s", p.ID, p.Status, p.Ref, p.CommitTitle)
	}
	return v
}

// Title implements View.
func (v *PipelinesView) Title() string { return "Pipelines" }

// Focus implements View: refreshes whatever is on screen, which is not always the
// pipeline list.
func (v *PipelinesView) Focus() tea.Cmd {
	if v.viewingJobs {
		if v.jobs.showingTrace() {
			return nil
		}
		return v.jobs.load()
	}
	return v.load()
}

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
		v.setItems(msg.Pipelines)
		return v.loadStages()

	case PipelineStagesLoadedMsg:
		v.stages = msg.Stages
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
		// A manual or pending job has nothing written yet; Enter on one used to look like a
		// key that did not work.
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
			v.paste(msg.Content)
		}
		return nil

	case tea.KeyMsg:
		return v.handleKey(msg)
	}
	return nil
}

// CapturingText implements TextCapturer: while the search is being typed, the shell
// must not read the letters as its own commands.
func (v *PipelinesView) CapturingText() bool { return !v.viewingJobs && v.capturing() }

func (v *PipelinesView) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	// The search owns the keys while it is open, so Esc clears it before it means "back".
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

	if key == keyToggleBox {
		v.stagesBox.toggle()
		return nil
	}

	if v.navigate(msg, v.height) {
		return nil
	}

	// Enter: load jobs for the selected pipeline
	if key == keyEnter {
		if p := v.selected(); p != nil {
			return v.jobs.open(p.ID)
		}
		return nil
	}

	// Pipeline actions
	switch key {
	case keyRetry:
		if p := v.selected(); p != nil {
			return confirmCmd(fmt.Sprintf("Retry pipeline #%d?", p.ID), v.retryPipeline())
		}
	case keyCancel:
		if p := v.selected(); p != nil {
			return confirmCmd(fmt.Sprintf("Cancel pipeline #%d?", p.ID), v.cancelPipeline())
		}
	case keyCopy:
		if p := v.selected(); p != nil {
			return copyRef(fmt.Sprintf("#%d", p.ID))
		}
	case keyCopyLink:
		if p := v.selected(); p != nil {
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

// Body implements View: the pipeline list, full width, laid out like every other list
// in the app.
func (v *PipelinesView) Body(width, height int) string {
	v.width = width
	v.height = height

	// Drilled into jobs: the shared panel owns both halves.
	if v.viewingJobs {
		return v.jobs.body(width, height)
	}

	// The marks say how far each pipeline got, but not which mark is which stage; the box
	// below names them for the highlighted row, out of the stages already in hand.
	if v.stagesBox.folded {
		return v.listBox(width, height)
	}
	return splitBody(width, height, len(v.stages[v.selectedID()])+1,
		v.listBox, panelBox(v.stagesTitle(), v.stageLines()))
}

// listBox renders the pipeline list itself.
func (v *PipelinesView) listBox(width, height int) string {
	return v.box(width, height, "Pipelines", func(p gitlab.Pipeline) listRow {
		return pipelineRow(p, v.stages[p.ID])
	}, true)
}

// selectedID is the highlighted pipeline's ID, or 0.
func (v *PipelinesView) selectedID() int {
	if p := v.selected(); p != nil {
		return p.ID
	}
	return 0
}

func (v *PipelinesView) stagesTitle() string {
	if p := v.selected(); p != nil {
		return fmt.Sprintf("Stages (#%d)", p.ID)
	}
	return "Stages"
}

// stageLines names the highlighted pipeline's stages, in the order it ran them — the
// same order as the marks on its row, and as the groups Enter opens.
func (v *PipelinesView) stageLines() []string {
	stages := v.stages[v.selectedID()]
	if len(stages) == 0 {
		if v.selected() == nil {
			return nil
		}
		return []string{components.HelpDescStyle.Render("No stages reported for this pipeline")}
	}

	width := 0
	for _, s := range stages {
		if n := lipgloss.Width(s.Name); n > width {
			width = n
		}
	}

	lines := make([]string, 0, len(stages))
	for _, s := range stages {
		jobs := fmt.Sprintf("%d jobs", s.Jobs)
		if s.Jobs == 1 {
			jobs = "1 job"
		}
		lines = append(lines, fmt.Sprintf("  %s %s  %s",
			components.StatusIcon(s.Status),
			components.PadRight(s.Name, width),
			components.MutedStyle.Render(jobs)))
	}
	return lines
}

// pipelineRow describes one pipeline row: what it built, how it went stage by stage, on
// which branch, and when it started.
func pipelineRow(p gitlab.Pipeline, stages []gitlab.Stage) listRow {
	title := p.CommitTitle
	if title == "" {
		title = p.Ref
	}
	kind, subject := splitConventional(title)
	status := p.Status
	if p.HasWarnings {
		// A failed allowed-to-fail job is not a plain success, the same way the dashboard's
		// commit list says so.
		status = components.StatusWarning
	}
	return listRow{
		kind:    kind,
		icon:    components.StatusIconPadded(status),
		subject: subject,
		marks:   stageMarks(stages),
		extra:   p.Ref,
		stamp:   commitStamp(p.CreatedAt),
	}
}

// stageMarks is one mark per stage, in order, in the status colours the rest of the app
// uses: the answer to "how far did it get.
func stageMarks(stages []gitlab.Stage) string {
	if len(stages) == 0 {
		return ""
	}
	marks := make([]string, 0, len(stages))
	for _, s := range stages {
		marks = append(marks, components.StatusIcon(s.Status))
	}
	return strings.Join(marks, " ")
}

// KeyHints implements View.
func (v *PipelinesView) KeyHints() []KeyHint {
	if v.viewingJobs {
		return v.jobs.keyHints()
	}
	return []KeyHint{
		{"Enter", "Jobs"},
		v.stagesBox.hint(),
		{"p", "Run"},
		{"R", "Retry"},
		{"C", "Cancel"},
		{"o", "Open"},
		{"y/Y", "Copy #/link"},
		v.search.hint(),
	}
}

// loadStages asks for the stages of every pipeline now on screen — one request for the
// page, and none at all once every finished pipeline in it is cached.
func (v *PipelinesView) loadStages() tea.Cmd {
	if v.ctx == nil || v.ctx.Project == nil || v.ctx.Client == nil || len(v.items) == 0 {
		return nil
	}
	client := v.ctx.Client
	path := v.ctx.Project.PathWithNamespace
	pipelines := v.items
	return func() tea.Msg {
		return PipelineStagesLoadedMsg{Stages: client.PipelineStages(path, pipelines)}
	}
}

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
	p := v.selected()
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
	p := v.selected()
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

// runRef returns the ref a new pipeline would run on (selected branch or the project
// default), or "" when unavailable.
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
	p := v.selected()
	if p == nil {
		return nil
	}
	cmd := openBrowserCmd(p.WebURL)
	if cmd == nil {
		return nil
	}
	return execBrowser(cmd)
}

// confirmCmd returns a command that asks the shell to confirm a destructive action
// before running it.
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

// listRows is how many list rows a view of the given height shows, i.e.
func listRows(height int) int {
	rows := height - 2
	if rows < 1 {
		rows = 1
	}
	return rows
}
