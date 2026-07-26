package views

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// testContext is a Context with a client and project that are never called.
func testContext(t *testing.T) *Context {
	t.Helper()
	client, err := gitlab.NewClient("token", "https://gitlab.example.com/api/v4", "gitlab.example.com")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return &Context{Client: client, Project: &gitlab.Project{ID: 1, DefaultBranch: "main"}}
}

func samplePanel() *jobsPanel {
	p := &jobsPanel{ctx: &Context{}}
	p.pipelineID = 722175
	p.setJobs([]gitlab.Job{
		{ID: 1, Name: "build", Stage: "build", Status: "success", Duration: 82},
		{ID: 2, Name: "unit", Stage: "test", Status: "failed", Duration: 12},
		{ID: 3, Name: "deploy", Stage: "deploy", Status: "manual"},
	})
	return p
}

func TestJobsPanel_GroupsByStageAndMapsCursor(t *testing.T) {
	p := samplePanel()
	items, jobToDisplay := p.items()

	// Each stage contributes a header row, so job indices and rows differ.
	if len(items) != 6 {
		t.Fatalf("want 6 rows (3 headers + 3 jobs), got %d: %v", len(items), items)
	}
	if jobToDisplay[0] != 1 || jobToDisplay[1] != 3 || jobToDisplay[2] != 5 {
		t.Errorf("job rows = %v, want [1 3 5]", jobToDisplay)
	}
	if p.cursorRow() != 1 {
		t.Errorf("cursorRow = %d, want the first job's row", p.cursorRow())
	}
	if !strings.Contains(items[1], "1m22s") {
		t.Errorf("expected the duration on the job row, got %q", items[1])
	}
}

func TestJobsPanel_NavigationSkipsHeaders(t *testing.T) {
	p := samplePanel()
	p.handleKey("j", 20)
	if p.cursor != 1 || p.selected().Name != "unit" {
		t.Errorf("cursor = %d (%v), want the second job", p.cursor, p.selected())
	}
	if p.cursorRow() != 3 {
		t.Errorf("cursorRow = %d, want row 3", p.cursorRow())
	}
}

func TestJobsPanel_OpenLogTakesTheNavigationKeys(t *testing.T) {
	// While reading a log, j/k scroll it rather than jumping between jobs; Esc
	// (handled by the host) closes it and hands the keys back to the list.
	p := samplePanel()
	p.setTrace(strings.Repeat("a log line\n", 50))

	p.handleKey("j", 20)
	if p.cursor != 0 {
		t.Errorf("cursor moved to %d while a log was open", p.cursor)
	}
	if p.traceScroll == 0 {
		t.Error("j should have scrolled the log")
	}

	p.closeTrace()
	p.handleKey("j", 20)
	if p.cursor != 1 {
		t.Errorf("cursor = %d, want 1 once the log is closed", p.cursor)
	}
}

func TestJobsPanel_LogScrollsWithTheUsualKeys(t *testing.T) {
	p := samplePanel()
	p.setTrace(strings.Repeat("a log line\n", 200))

	p.handleKey("j", 20)
	if p.traceScroll != 1 {
		t.Errorf("traceScroll = %d, want 1", p.traceScroll)
	}
	p.handleKey(".", 20)
	if p.traceScroll <= 1 {
		t.Error("page down should move further than a line")
	}
	p.handleKey("<", 20)
	if p.traceScroll != 0 {
		t.Errorf("< should return to the top, got %d", p.traceScroll)
	}

	// The cursor must not have moved: those keys scrolled the log.
	if p.cursor != 0 {
		t.Errorf("cursor moved to %d while scrolling a log", p.cursor)
	}
}

func TestJobsPanel_ActionsConfirmFirst(t *testing.T) {
	for _, tc := range []struct{ key, want string }{
		{"R", "Retry job"},
		{"C", "Cancel job"},
		{"p", "Play job"},
	} {
		p := samplePanel()
		cmd, consumed := p.handleKey(tc.key, 20)
		if !consumed {
			t.Fatalf("%s should be handled by the panel", tc.key)
		}
		if cmd == nil {
			t.Fatalf("%s should ask for confirmation", tc.key)
		}
		msg, ok := cmd().(ConfirmMsg)
		if !ok {
			t.Fatalf("%s: expected ConfirmMsg, got %T", tc.key, cmd())
		}
		if !strings.Contains(msg.Prompt, tc.want) || !strings.Contains(msg.Prompt, "build") {
			t.Errorf("%s prompt = %q, want it to name the action and the job", tc.key, msg.Prompt)
		}
	}
}

func TestJobsPanel_PlayOnlyAppliesToManualJobs(t *testing.T) {
	p := samplePanel() // cursor is on "build", a finished job
	cmd, _ := p.handleKey("p", 20)
	confirm := cmd().(ConfirmMsg)

	// Confirming a non-manual job must explain rather than call the API.
	msg := confirm.Action()
	status, ok := msg.(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", msg)
	}
	if !status.IsErr || !strings.Contains(status.Text, "Only manual jobs") {
		t.Errorf("status = %+v", status)
	}
}

func TestJobsPanel_UnknownKeysFallThrough(t *testing.T) {
	// The host view must keep its own bindings, e.g. b for the branch picker.
	p := samplePanel()
	if _, consumed := p.handleKey("b", 20); consumed {
		t.Error("the panel must not swallow keys it has no use for")
	}
}

func TestJobsPanel_EmptyPanelIsSafe(t *testing.T) {
	p := &jobsPanel{ctx: &Context{}}
	for _, key := range []string{"j", "k", "enter", "R", "C", "p", "o"} {
		if cmd, _ := p.handleKey(key, 20); cmd != nil {
			cmd() // must not panic
		}
	}
	if got := p.detail(80, 20); got != "No jobs" {
		t.Errorf("detail = %q, want %q", got, "No jobs")
	}
}

func TestJobsPanel_OpeningAnotherPipelineResetsState(t *testing.T) {
	p := samplePanel()
	p.cursor = 2
	p.setTrace("log")

	p.open(999) // a different pipeline
	if p.cursor != 0 || p.jobs != nil || p.showingTrace() {
		t.Errorf("state carried over: cursor=%d jobs=%v trace=%v", p.cursor, p.jobs, p.showingTrace())
	}
	if p.pipelineID != 999 {
		t.Errorf("pipelineID = %d, want 999", p.pipelineID)
	}
}

func TestJobsPanel_BoxTitles(t *testing.T) {
	p := samplePanel()
	if got := p.listTitle(); !strings.Contains(got, "722175") {
		t.Errorf("list title = %q, want the pipeline number", got)
	}
	if got := p.detailTitle(); !strings.Contains(got, "build") {
		t.Errorf("detail title = %q, want the job name", got)
	}

	p.setTrace("log")
	if got := p.detailTitle(); !strings.Contains(got, "Log") {
		t.Errorf("detail title = %q, want it to name the log", got)
	}
	// The list is still the list, not a second copy of the log's title.
	if got := p.listTitle(); strings.Contains(got, "Log") {
		t.Errorf("list title = %q, should stay the job list", got)
	}
}

func TestJobsPanel_LogViewStripsNoiseAndClamps(t *testing.T) {
	p := samplePanel()
	p.setTrace("section_start:1234:step\n\x1b[31mred line\x1b[0m\r\nplain\nsection_end:1234:step\n")
	p.traceScroll = 1000 // beyond the end

	out := p.traceView(80, 10)
	if strings.Contains(out, "section_start") || strings.Contains(out, "section_end") {
		t.Errorf("section markers should be stripped:\n%s", out)
	}
	if strings.Contains(out, "\x1b[31m") {
		t.Error("ANSI colours from the runner should be stripped")
	}
	if out == "" {
		t.Error("an over-scrolled log should clamp to the end, not go blank")
	}
}

func TestPipelinesView_EnterOpensThePanel(t *testing.T) {
	v := NewPipelinesView(&Context{})
	v.width, v.height = 120, 30
	v.pipelines = []gitlab.Pipeline{{ID: 722175, Status: "failed", Ref: "main"}}

	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if v.jobs.pipelineID != 722175 {
		t.Errorf("expected the panel on pipeline 722175, got %d", v.jobs.pipelineID)
	}
}

func TestPipelinesView_EscUnwindsLogThenJobsThenList(t *testing.T) {
	v := NewPipelinesView(&Context{})
	v.width, v.height = 120, 30
	v.pipelines = []gitlab.Pipeline{{ID: 722175, Status: "failed"}}
	v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	v.Update(JobsLoadedMsg{Jobs: []gitlab.Job{{ID: 1, Name: "build", Stage: "build"}}})
	v.Update(JobTraceLoadedMsg{Trace: "log output"})

	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
	v.Update(esc)
	if v.jobs.showingTrace() {
		t.Fatal("the first Esc should close the log")
	}
	if !v.viewingJobs {
		t.Fatal("the job list should still be open")
	}
	v.Update(esc)
	if v.viewingJobs || v.jobs.pipelineID != 0 {
		t.Error("the second Esc should return to the pipeline list")
	}
}

// TestCommitPage_RoutesJobMessagesToThePanel locks down the wiring: the panel is
// useless if its host swallows the job list, the log, or an action's result.
func TestCommitPage_RoutesJobMessagesToThePanel(t *testing.T) {
	hosts := map[string]interface {
		Update(tea.Msg) tea.Cmd
	}{}

	// A real (unused) client and project, so the reload after an action has
	// something to return. No command is executed here.
	ctx := testContext(t)

	commits := NewCommitsView(ctx)
	commits.width, commits.height = 120, 30
	commits.commits = []gitlab.Commit{{ShortID: "38333fa4"}}
	hosts["commits"] = commits

	overview := NewOverviewView(ctx)
	overview.width, overview.height = 120, 30
	overview.commits = []gitlab.Commit{{ShortID: "38333fa4"}}
	hosts["overview"] = overview

	for name, host := range hosts {
		t.Run(name, func(t *testing.T) {
			var detail *commitDetail
			switch h := host.(type) {
			case *CommitsView:
				detail = &h.detail
			case *OverviewView:
				detail = &h.detail
			}

			host.Update(enterKey) // open the commit page
			detail.sha = "38333fa4"
			host.Update(loadedDetail("38333fa4"))

			// The jobs the page loaded are the panel's own, without a second fetch.
			if len(detail.jobs.jobs) != 3 {
				t.Fatalf("the page's jobs never reached the panel: %v", detail.jobs.jobs)
			}

			host.Update(enterKey) // focus moves into them
			if detail.focus != focusJobs {
				t.Fatal("Enter should move the focus into the jobs")
			}

			host.Update(JobsLoadedMsg{Jobs: []gitlab.Job{
				{ID: 1, Name: "build", Stage: "build", Status: "success"},
			}})
			if len(detail.jobs.jobs) != 1 {
				t.Fatalf("a refreshed job list never reached the panel: %v", detail.jobs.jobs)
			}

			host.Update(JobTraceLoadedMsg{Trace: "log output"})
			if !detail.jobs.showingTrace() {
				t.Error("the log never reached the panel")
			}

			// An action's result must refresh the jobs behind it.
			if cmd := host.Update(JobActionDoneMsg{Text: "Retried job 'build'"}); cmd == nil {
				t.Error("expected a reload after a job action")
			}
		})
	}
}

func TestCommitPage_EscUnwindsLogThenJobsFocusThenPage(t *testing.T) {
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.commits = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(loadedDetail("38333fa4")) // carries the jobs
	v.Update(enterKey)                 // focus moves into the jobs on the page
	v.Update(JobTraceLoadedMsg{Trace: "log"})

	v.Update(escKey)
	if v.detail.jobs.showingTrace() {
		t.Fatal("the first Esc should close the log")
	}
	if v.detail.focus != focusJobs {
		t.Fatal("closing the log should leave the focus in the jobs")
	}

	v.Update(escKey)
	if v.detail.focus != focusPage {
		t.Fatal("the second Esc should hand the keys back to the page")
	}
	if !v.detail.active {
		t.Fatal("the commit page should still be open")
	}

	v.Update(escKey)
	if v.detail.active {
		t.Error("the third Esc should return to the commit list")
	}
}

func TestCommitPage_JobActionsWorkFromTheCommit(t *testing.T) {
	// The point of sharing the panel: a pipeline is controllable from a commit.
	v := NewCommitsView(&Context{})
	v.width, v.height = 120, 30
	v.commits = []gitlab.Commit{{ShortID: "38333fa4"}}
	v.Update(enterKey)
	v.detail.sha = "38333fa4"
	v.Update(loadedDetail("38333fa4"))
	v.Update(enterKey)
	v.Update(JobsLoadedMsg{Jobs: []gitlab.Job{
		{ID: 1, Name: "build", Stage: "build", Status: "failed"},
	}})

	cmd := v.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	if cmd == nil {
		t.Fatal("R should ask to retry the job")
	}
	msg, ok := cmd().(ConfirmMsg)
	if !ok {
		t.Fatalf("expected ConfirmMsg, got %T", cmd())
	}
	if !strings.Contains(msg.Prompt, "Retry job") || !strings.Contains(msg.Prompt, "build") {
		t.Errorf("prompt = %q", msg.Prompt)
	}
}
