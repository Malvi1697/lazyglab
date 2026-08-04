package gitlab

import (
	"fmt"

	gogitlab "gitlab.com/gitlab-org/api/client-go"

	"github.com/Malvi1697/lazyglab/internal/util"
)

// ListTodos returns the current user's pending to-dos, across every project.
func (c *Client) ListTodos() ([]Todo, error) {
	opts := &gogitlab.ListTodosOptions{
		State:       gogitlab.Ptr("pending"),
		ListOptions: gogitlab.ListOptions{PerPage: 100},
	}

	apiTodos, _, err := c.api.Todos.ListTodos(opts)
	if err != nil {
		return nil, err
	}

	todos := make([]Todo, 0, len(apiTodos))
	for _, t := range apiTodos {
		if t == nil {
			continue
		}
		todo := Todo{
			ID:     int(t.ID),
			Action: util.StripANSI(string(t.ActionName)),
			Target: util.StripANSI(string(t.TargetType)),
			Body:   util.StripANSI(t.Body),
			State:  util.StripANSI(t.State),
			WebURL: util.StripANSI(t.TargetURL),
		}
		if t.CreatedAt != nil {
			todo.CreatedAt = *t.CreatedAt
		}
		if t.Author != nil {
			todo.Author = util.StripANSI(t.Author.Username)
		}
		if t.Project != nil {
			todo.ProjectPath = util.StripANSI(t.Project.PathWithNamespace)
		}
		if t.Target != nil {
			todo.Title = util.StripANSI(t.Target.Title)
			todo.Reference = reference(string(t.TargetType), int(t.Target.IID))
			todo.TargetState = util.StripANSI(t.Target.State)
		}
		// A to-do whose target carries no title (a design, say) still has the body GitLab
		// wrote for it, which is better than an empty row.
		if todo.Title == "" {
			todo.Title = todo.Body
		}
		todos = append(todos, todo)
	}
	return todos, nil
}

// reference is how GitLab writes a target's number: !5 for a merge request, #5 for an
// issue, nothing for the target types that have no number.
func reference(targetType string, iid int) string {
	if iid == 0 {
		return ""
	}
	switch targetType {
	case "MergeRequest":
		return fmt.Sprintf("!%d", iid)
	case "Issue", "WorkItem":
		return fmt.Sprintf("#%d", iid)
	default:
		return fmt.Sprintf("%d", iid)
	}
}

// MarkTodoDone clears one to-do.
func (c *Client) MarkTodoDone(id int) error {
	_, err := c.api.Todos.MarkTodoAsDone(int64(id))
	return err
}

// MarkAllTodosDone clears the whole to-do list.
func (c *Client) MarkAllTodosDone() error {
	_, err := c.api.Todos.MarkAllTodosAsDone()
	return err
}
