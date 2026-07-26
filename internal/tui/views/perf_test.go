package views

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

// What one frame costs. Every keypress redraws the whole body, so anything that
// re-derives its content from scratch here is paid again on every single key.

// benchCommits is one page of commits: what a real list holds.
func benchCommits() []gitlab.Commit {
	out := make([]gitlab.Commit, 50)
	for i := range out {
		out[i] = gitlab.Commit{
			ID:         fmt.Sprintf("%040d", i),
			ShortID:    fmt.Sprintf("%08d", i),
			Title:      fmt.Sprintf("feat(scope): change number %d in the codebase", i),
			AuthorName: "Some Author",
			CreatedAt:  time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}
	return out
}

func benchPipelines(n int) []gitlab.Pipeline {
	out := make([]gitlab.Pipeline, n)
	for i := range out {
		out[i] = gitlab.Pipeline{
			ID: 100 + i, Status: "success", Ref: "main",
			SHA:         fmt.Sprintf("%040d", i),
			CommitTitle: fmt.Sprintf("feat(scope): change number %d", i),
			CreatedAt:   time.Now().Add(-time.Duration(i) * time.Hour),
		}
	}
	return out
}

func benchJobs(n int) []gitlab.Job {
	out := make([]gitlab.Job, n)
	for i := range out {
		out[i] = gitlab.Job{
			ID: i, Name: fmt.Sprintf("build:package-%d", i),
			Stage: []string{"build", "test", "deploy"}[i%3], Status: "success", Duration: 42,
		}
	}
	return out
}

func BenchmarkOverviewBody(b *testing.B) {
	v := NewOverviewView(&Context{})
	v.commits = benchCommits()
	v.pipelines = benchPipelines(30)
	v.mrs = []gitlab.MergeRequest{{IID: 1, Title: "MR"}}
	v.issues = []gitlab.Issue{{IID: 1, Title: "Issue"}}

	for b.Loop() {
		v.Body(160, 45)
	}
}

func BenchmarkOverviewBodyWhileSearching(b *testing.B) {
	v := NewOverviewView(&Context{})
	v.commits = benchCommits()
	v.pipelines = benchPipelines(30)
	v.search.filter.Query = "number 4"

	for b.Loop() {
		v.Body(160, 45)
	}
}

func BenchmarkJobsPanelBody(b *testing.B) {
	p := &jobsPanel{ctx: &Context{}, pipelineID: 7}
	p.setJobs(benchJobs(80))

	for b.Loop() {
		p.body(160, 45)
	}
}

func BenchmarkJobLogScroll(b *testing.B) {
	// A CI log of a few thousand lines, scrolled one row at a time. Every frame
	// re-derives the whole thing unless something remembers it.
	var log strings.Builder
	for i := 0; i < 4000; i++ {
		fmt.Fprintf(&log, "\x1b[32m[%04d]\x1b[0m running step %d of the build with some detail\n", i, i)
	}
	p := &jobsPanel{ctx: &Context{}, pipelineID: 7}
	p.setJobs(benchJobs(3))
	p.setTrace(log.String())

	i := 0
	for b.Loop() {
		p.traceScroll = i % 200
		i++
		p.traceView(160, 45)
	}
}

func BenchmarkCommitPageBody(b *testing.B) {
	page := newCommitDetail(&Context{})
	d := &page
	d.active = true
	d.commit = &gitlab.Commit{
		ID: "abc", ShortID: "abc1234", Title: "feat: something",
		Message: "feat: something\n\n" + strings.Repeat("a paragraph of the message body\n", 20),
	}
	d.pipelines = benchPipelines(1)
	d.diffs = make([]gitlab.FileDiff, 40)
	for i := range d.diffs {
		d.diffs[i] = gitlab.FileDiff{NewPath: fmt.Sprintf("pkg/file%d.go", i), Added: 3, Removed: 1}
	}
	d.jobs.adopt(1, benchJobs(40))

	for b.Loop() {
		d.body(160, 45)
	}
}

func BenchmarkDiffScroll(b *testing.B) {
	page := newCommitDetail(&Context{})
	d := &page
	d.active = true
	d.reading = true
	d.commit = &gitlab.Commit{ID: "abc", ShortID: "abc1234", Title: "t"}

	var diff strings.Builder
	diff.WriteString("@@ -1,400 +1,400 @@\n")
	for i := 0; i < 1200; i++ {
		fmt.Fprintf(&diff, " \tresult := compute(value%d, \"literal %d\") // a comment\n", i, i)
	}
	d.diffs = []gitlab.FileDiff{{NewPath: "pkg/thing.go", Diff: diff.String()}}

	i := 0
	for b.Loop() {
		d.diffScroll = i % 200
		i++
		d.body(160, 45)
	}
}

func BenchmarkCommitRowStyling(b *testing.B) {
	// How much of a frame is styling rows that the window will never show.
	v := NewOverviewView(&Context{})
	v.commits = benchCommits()
	v.pipelines = benchPipelines(30)
	visible := v.visible()

	for b.Loop() {
		for i := range visible {
			_ = v.commitRow(visible[i])
		}
	}
}

func BenchmarkVisibleSlice(b *testing.B) {
	v := NewOverviewView(&Context{})
	v.commits = benchCommits()
	v.search.filter.Query = "number 4"

	for b.Loop() {
		v.visible()
	}
}
