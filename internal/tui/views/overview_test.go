package views

import (
	"testing"

	"github.com/Malvi1697/lazyglab/internal/gitlab"
)

func TestCommitStatus(t *testing.T) {
	pipelines := []gitlab.Pipeline{
		{SHA: "f138bae6729508b923de684d5a8e4f8a72eda3f2", Status: "success"},
	}

	if got := commitStatus("f138bae6", pipelines); got != "success" {
		t.Errorf(`commitStatus("f138bae6", pipelines) = %q, want "success"`, got)
	}
	if got := commitStatus("deadbeef", pipelines); got != "" {
		t.Errorf(`commitStatus("deadbeef", pipelines) = %q, want ""`, got)
	}
	if got := commitStatus("", pipelines); got != "" {
		t.Errorf(`commitStatus("", pipelines) = %q, want ""`, got)
	}
}
