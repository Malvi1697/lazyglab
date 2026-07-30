package views

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// The API budget of each user action, measured rather than guessed.
//
// A cockpit that refreshes itself every 30 seconds is only welcome on someone
// else's GitLab if a refresh is cheap, so these tests count the requests one
// action really makes and fail when that count grows.

// apiRecorder is a stand-in GitLab that answers plausibly and counts what was
// asked of it, by shape of path rather than by exact id.
type apiRecorder struct {
	mu     sync.Mutex
	counts map[string]int
	total  int
}

var (
	hexSegment = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
	numSegment = regexp.MustCompile(`^[0-9]+$`)
)

// shape turns "/api/v4/projects/1/pipelines/77/jobs" into
// "/projects/:id/pipelines/:id/jobs" so repeated calls group together.
func shape(path string) string {
	path = strings.TrimPrefix(path, "/api/v4")
	parts := strings.Split(path, "/")
	for i, p := range parts {
		switch {
		case numSegment.MatchString(p):
			parts[i] = ":id"
		case hexSegment.MatchString(p):
			parts[i] = ":sha"
		}
	}
	return strings.Join(parts, "/")
}

func (r *apiRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	r.counts[shape(req.URL.Path)]++
	r.total++
	r.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(bodyFor(req.URL.Path)))
}

func (r *apiRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counts = map[string]int{}
	r.total = 0
}

// report is the counts as a stable, readable table for the test log.
func (r *apiRecorder) report() (int, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	keys := make([]string, 0, len(r.counts))
	for k := range r.counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "\n    %3d  %s", r.counts[k], k)
	}
	return r.total, b.String()
}

// The three commits, their three pipelines and the SHAs that tie them together.
var costSHAs = []string{
	"aaaaaaa1111111111111111111111111111111a1",
	"bbbbbbb2222222222222222222222222222222b2",
	"ccccccc3333333333333333333333333333333c3",
}

func bodyFor(path string) string {
	switch shape(path) {
	case "/projects/:id/repository/commits":
		var items []string
		for i, sha := range costSHAs {
			items = append(items, fmt.Sprintf(
				`{"id":%q,"short_id":%q,"title":"commit %d","author_name":"A",
				  "created_at":"2026-07-20T08:00:00Z","web_url":"https://gl/c/%s"}`,
				sha, sha[:8], i, sha))
		}
		return "[" + strings.Join(items, ",") + "]"

	case "/projects/:id/repository/commits/:sha":
		return fmt.Sprintf(`{"id":%q,"short_id":%q,"title":"commit","message":"commit\n\nbody",
			"author_name":"A","created_at":"2026-07-20T08:00:00Z","parent_ids":[]}`,
			costSHAs[0], costSHAs[0][:8])

	case "/projects/:id/repository/commits/:sha/diff":
		return `[{"old_path":"a.py","new_path":"a.py","diff":"@@ -1 +1 @@\n-x\n+y\n"}]`

	case "/projects/:id/repository/commits/:sha/refs":
		return `[{"type":"branch","name":"main"}]`

	case "/projects/:id/repository/commits/:sha/merge_requests":
		return `[]`

	case "/projects/:id/pipelines":
		var items []string
		for i, sha := range costSHAs {
			items = append(items, fmt.Sprintf(
				`{"id":%d,"sha":%q,"ref":"main","status":"success","web_url":"https://gl/p/%d",
				  "created_at":"2026-07-20T08:00:00Z","updated_at":"2026-07-20T08:05:00Z"}`,
				70+i, sha, 70+i))
		}
		return "[" + strings.Join(items, ",") + "]"

	case "/api/graphql":
		// The stages query, answered for whichever pipelines were asked about. The
		// fixture always returns the same three stages; what the test cares about is
		// how often it is asked.
		var nodes []string
		for i := range costSHAs {
			nodes = append(nodes, fmt.Sprintf(
				`{"id":"gid://gitlab/Ci::Pipeline/%d","stages":{"nodes":[
				  {"name":"lint","jobs":{"nodes":[{"status":"SUCCESS"}]}},
				  {"name":"test","jobs":{"nodes":[{"status":"SUCCESS"}]}}]}}`, 70+i))
		}
		return `{"data":{"project":{"pipelines":{"nodes":[` + strings.Join(nodes, ",") + `]}}}}`

	case "/projects/:id/pipelines/:id":
		return `{"id":70,"sha":"` + costSHAs[0] + `","ref":"main","status":"success",
			"detailed_status":{"icon":"status_warning","text":"passed","label":"passed with warnings","group":"success-with-warnings"}}`

	case "/projects/:id/pipelines/:id/jobs":
		return `[{"id":1,"name":"build","stage":"build","status":"success","duration":12},
			{"id":2,"name":"test","stage":"test","status":"success","duration":30}]`

	case "/projects/:id/merge_requests":
		// Two of them, so a test can step from one to the other.
		return `[{"id":1042,"iid":42,"title":"MR","author":{"username":"a"},"source_branch":"f","target_branch":"main","state":"opened","web_url":"https://gl/mr/42"},
			{"id":1012,"iid":12,"title":"Another MR","author":{"username":"b"},"source_branch":"g","target_branch":"main","state":"opened","web_url":"https://gl/mr/12"}]`

	case "/projects/:id/issues":
		// The id matters: client-go's Issue unmarshaller reflects on it and panics
		// when it is missing.
		return `[{"id":1007,"iid":7,"title":"Issue","author":{"username":"a"},"state":"opened","web_url":"https://gl/i/7"}]`

	case "/projects/:id/merge_requests/:id":
		return `{"id":1042,"iid":42,"title":"MR","author":{"username":"a"},
			"source_branch":"f","target_branch":"main","state":"opened","sha":"` + costSHAs[0] + `",
			"detailed_merge_status":"mergeable","web_url":"https://gl/mr/42",
			"pipeline":{"id":70,"status":"success","web_url":"https://gl/p/70"}}`

	case "/projects/:id/merge_requests/:id/diffs":
		return `[{"old_path":"a.py","new_path":"a.py","diff":"@@ -1 +1 @@\n-x\n+y\n"}]`

	case "/projects/:id/merge_requests/:id/approvals":
		return `{"id":1042,"iid":42,"approved":false,"approvals_required":2,"approvals_left":1,
			"approved_by":[{"user":{"username":"dave"}}],"user_can_approve":true}`

	case "/projects/:id/merge_requests/:id/notes", "/projects/:id/issues/:id/notes":
		return `[{"id":9,"author":{"username":"alice"},"body":"Looks good","system":false,
			"created_at":"2026-07-20T09:00:00Z"}]`

	case "/todos":
		return `[{"id":1,"action_name":"review_requested","target_type":"MergeRequest",
			"target":{"iid":42,"title":"MR"},"target_url":"https://gl/mr/42","body":"MR","state":"pending"}]`
	}
	return `[]`
}

// costHarness is a recorder plus a view context wired to it.
func costHarness(t *testing.T) (*apiRecorder, *Context, func()) {
	t.Helper()
	rec := &apiRecorder{counts: map[string]int{}}
	srv := httptest.NewServer(rec)
	client, err := gitlab.NewClient("token", srv.URL+"/api/v4", "test")
	if err != nil {
		srv.Close()
		t.Fatalf("building the client: %v", err)
	}
	ctx := &Context{Client: client, Project: &gitlab.Project{ID: 1, DefaultBranch: "main"}}
	return rec, ctx, srv.Close
}

// drain runs a command and everything it batches, feeding each resulting message
// back into the view — which is what the shell does, and what makes follow-up
// requests happen.
func drain(v View, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case nil:
		return
	case tea.BatchMsg:
		for _, c := range msg {
			drain(v, c)
		}
	default:
		drain(v, v.Update(msg))
	}
}

// cost measures one action, logging the breakdown.
func cost(t *testing.T, rec *apiRecorder, label string, action func()) int {
	t.Helper()
	rec.reset()
	action()
	total, table := rec.report()
	t.Logf("%-34s %3d requests%s", label, total, table)
	return total
}

func TestAPICost_OverviewRefresh(t *testing.T) {
	rec, ctx, done := costHarness(t)
	defer done()

	v := NewDashboardView(ctx)
	v.width, v.height = 120, 40

	first := cost(t, rec, "Overview: first load", func() { drain(v, v.Focus()) })
	second := cost(t, rec, "Overview: auto-refresh (30s)", func() { drain(v, v.Focus()) })

	// Four lists — commits, pipelines, merge requests, issues — is the floor.
	if first < 4 {
		t.Fatalf("first load = %d requests, expected at least the four lists", first)
	}
	// Nothing about the three commits changed, so a refresh must not pay for their
	// titles and verdicts all over again.
	if second > 5 {
		t.Errorf("a refresh costs %d requests, want no more than the 4 lists plus a margin;\n"+
			"immutable per-commit data is being refetched every 30 seconds", second)
	}
}

func TestAPICost_PipelineListDoesNotFanOutPerRow(t *testing.T) {
	rec, ctx, done := costHarness(t)
	defer done()

	v := NewPipelinesView(ctx)
	v.width, v.height = 120, 40

	first := cost(t, rec, "Pipelines: first load", func() { drain(v, v.Focus()) })
	second := cost(t, rec, "Pipelines: auto-refresh (30s)", func() { drain(v, v.Focus()) })

	// Three pipelines here stand in for the thirty a real project has: what matters
	// is that the cost does not scale with the number of rows on every refresh.
	if second > 2 {
		t.Errorf("refreshing 3 pipelines costs %d requests (first load %d); with 30 rows that is "+
			"a per-row fan-out on every tick", second, first)
	}

	// The list is one request and the stage marks are one more for the whole page —
	// the reason they come from GraphQL rather than from a jobs call per row.
	if got := rec.counts["/api/graphql"]; got > 1 {
		t.Errorf("the stages cost %d requests for one refresh, want at most 1", got)
	}
}

func TestAPICost_FinishedPipelinesStagesAreAskedForOnce(t *testing.T) {
	// A finished pipeline's stages cannot change, so the thirty-second refresh should
	// stop asking about them entirely.
	rec, ctx, done := costHarness(t)
	defer done()

	v := NewPipelinesView(ctx)
	v.width, v.height = 120, 40

	drain(v, v.Focus())
	rec.reset()
	drain(v, v.Focus())

	if got := rec.counts["/api/graphql"]; got != 0 {
		t.Errorf("a refresh asked for stages %d times, want none once they are cached%s",
			got, func() string { _, table := rec.report(); return table }())
	}
}

func TestAPICost_OpeningAndSteppingCommits(t *testing.T) {
	rec, ctx, done := costHarness(t)
	defer done()

	v := NewDashboardView(ctx)
	v.width, v.height = 120, 40
	drain(v, v.Focus())

	enter := tea.KeyPressMsg{Code: tea.KeyEnter}
	open := cost(t, rec, "Commit page: open", func() { drain(v, v.Update(enter)) })

	stepIn := tea.KeyPressMsg{Code: 'l', Text: "l"}
	step := cost(t, rec, "Commit page: step to the next", func() { drain(v, v.Update(stepIn)) })

	back := cost(t, rec, "Commit page: step back", func() {
		drain(v, v.Update(tea.KeyPressMsg{Code: 'h', Text: "h"}))
	})

	if open > 6 {
		t.Errorf("opening a commit page costs %d requests, want at most 6", open)
	}
	if step > open {
		t.Errorf("stepping costs %d requests, more than opening (%d)", step, open)
	}
	// Stepping back lands on a commit whose page was fetched a keypress ago;
	// walking a list of commits is exactly the thing people do twice.
	if back != 0 {
		t.Errorf("stepping back to an already-fetched commit costs %d requests, want none", back)
	}
}

func TestAPICost_TodosAndOtherListsAreOneRequest(t *testing.T) {
	rec, ctx, done := costHarness(t)
	defer done()

	for _, tc := range []struct {
		name string
		view View
	}{
		{"Todos", NewTodosView(ctx)},
		{"Merge Requests", NewMRsView(ctx)},
		{"Issues", NewIssuesView(ctx)},
	} {
		v := tc.view
		if got := cost(t, rec, tc.name+": load", func() { drain(v, v.Focus()) }); got != 1 {
			t.Errorf("%s costs %d requests, want exactly 1", tc.name, got)
		}
	}
}

func TestAPICost_HoldingTheStepKeyFetchesOnlyWhereYouStop(t *testing.T) {
	// A page is six requests and a held key steps faster than any of them can
	// answer. Walking through three commits must cost one page, not three.
	rec, ctx, done := costHarness(t)
	defer done()

	v := NewDashboardView(ctx)
	v.width, v.height = 120, 40
	drain(v, v.Focus())
	drain(v, v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})) // open the first commit

	rec.reset()
	step := tea.KeyPressMsg{Code: 'l', Text: "l"}
	var pending []tea.Cmd
	for i := 0; i < 2; i++ {
		pending = append(pending, v.Update(step))
	}

	// Nothing has been asked yet: the steps are still settling.
	if total, table := rec.report(); total != 0 {
		t.Errorf("stepping asked for %d requests before settling%s", total, table)
	}
	// The commit on screen is the one we walked to, even though its detail is not in.
	if v.detail.commit == nil || v.detail.commit.ShortID != costSHAs[2][:8] {
		t.Errorf("page shows %v, want the commit stepped to", v.detail.commit)
	}

	// Every step's timer fires; only the last one still points at what is on screen.
	for _, cmd := range pending {
		drain(v, cmd)
	}
	total, table := rec.report()
	t.Logf("%-34s %3d requests%s", "Two steps, then settle", total, table)
	if total > 6 {
		t.Errorf("two steps cost %d requests, want one page's worth%s", total, table)
	}
	if total == 0 {
		t.Error("the commit we settled on was never fetched")
	}
}

func TestAPICost_OpeningIsNotDelayed(t *testing.T) {
	// Enter means "show me this one"; only stepping waits.
	rec, ctx, done := costHarness(t)
	defer done()

	v := NewDashboardView(ctx)
	v.width, v.height = 120, 40
	drain(v, v.Focus())

	rec.reset()
	cmd := v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	drain(v, cmd)
	if total, _ := rec.report(); total == 0 {
		t.Error("Enter should fetch the commit page at once")
	}
}

func TestAPICost_MergeRequestPage(t *testing.T) {
	// The page is four calls: the merge request, its approvals, its diffs and its
	// pipeline (which brings a fifth for that pipeline's jobs). Stepping back to one
	// already read costs nothing, as on the commit page.
	rec, ctx, done := costHarness(t)
	defer done()

	v := NewMRsView(ctx)
	v.width, v.height = 160, 45
	drain(v, v.Focus())

	open := cost(t, rec, "MR page: open", func() {
		drain(v, v.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))
	})
	if open > 6 {
		t.Errorf("opening a merge request costs %d requests, want at most 6", open)
	}

	step := cost(t, rec, "MR page: step to the next", func() {
		drain(v, v.Update(tea.KeyPressMsg{Code: 'l', Text: "l"}))
	})
	if step > open {
		t.Errorf("stepping costs %d requests, more than opening (%d)", step, open)
	}

	back := cost(t, rec, "MR page: step back", func() {
		drain(v, v.Update(tea.KeyPressMsg{Code: 'h', Text: "h"}))
	})
	if back != 0 {
		t.Errorf("stepping back to an already-fetched merge request costs %d requests, want none", back)
	}
}

func TestAPICost_RefreshFollowsWhatIsOnScreen(t *testing.T) {
	// The tick used to reload the list behind whatever you had open, so a pipeline
	// you sat watching never changed — and on the Pipelines view it threw you out
	// of the jobs panel every thirty seconds.
	rec, ctx, done := costHarness(t)
	defer done()

	t.Run("pipelines: the jobs, not the list", func(t *testing.T) {
		v := NewPipelinesView(ctx)
		v.width, v.height = 120, 40
		drain(v, v.Focus())
		drain(v, v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})) // into the jobs

		rec.reset()
		drain(v, v.Focus())
		_, table := rec.report()
		if !strings.Contains(table, "/pipelines/:id/jobs") {
			t.Errorf("a refresh asked for%s, want the jobs on screen", table)
		}
		if strings.Contains(table, "  /projects/:id/pipelines\n") {
			t.Errorf("a refresh asked for%s, want no list reload behind the panel", table)
		}
		if !v.viewingJobs {
			t.Error("a refresh must not throw you out of the jobs panel")
		}
	})

	t.Run("merge request: the page", func(t *testing.T) {
		v := NewMRsView(ctx)
		v.width, v.height = 160, 45
		drain(v, v.Focus())
		drain(v, v.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))

		rec.reset()
		drain(v, v.Focus())
		_, table := rec.report()
		if !strings.Contains(table, "/merge_requests/:id\n") {
			t.Errorf("a refresh asked for%s, want the open merge request", table)
		}
	})

	t.Run("commit page: the commit", func(t *testing.T) {
		v := NewDashboardView(ctx)
		v.width, v.height = 160, 45
		drain(v, v.Focus())
		drain(v, v.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))

		rec.reset()
		drain(v, v.Focus())
		_, table := rec.report()
		if !strings.Contains(table, "/repository/commits/:sha\n") {
			t.Errorf("a refresh asked for%s, want the open commit", table)
		}
		if strings.Contains(table, "/projects/:id/issues") {
			t.Errorf("a refresh asked for%s, want the page rather than Overview's lists", table)
		}
	})

	t.Run("issue page: its discussion", func(t *testing.T) {
		v := NewIssuesView(ctx)
		v.width, v.height = 160, 45
		drain(v, v.Focus())
		drain(v, v.Update(tea.KeyPressMsg{Code: tea.KeyEnter}))

		rec.reset()
		drain(v, v.Focus())
		if _, table := rec.report(); !strings.Contains(table, "/issues/:id/notes") {
			t.Errorf("a refresh asked for%s, want the discussion", table)
		}
	})
}

func TestAPICost_NothingIsRefetchedUnderSomeoneReading(t *testing.T) {
	// A refetch would move what someone is halfway through: the diff's scroll, the
	// log's, the thread's.
	rec, ctx, done := costHarness(t)
	defer done()

	v := NewMRsView(ctx)
	v.width, v.height = 160, 45
	drain(v, v.Focus())
	drain(v, v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})) // the page
	drain(v, v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})) // into the changed files
	drain(v, v.Update(tea.KeyPressMsg{Code: tea.KeyEnter})) // reading a diff

	if !v.detail.reading {
		t.Fatal("expected a diff on screen")
	}
	if got := cost(t, rec, "Refresh while reading a diff", func() { drain(v, v.Focus()) }); got != 0 {
		t.Errorf("a refresh cost %d requests while a diff was being read, want none", got)
	}
}
