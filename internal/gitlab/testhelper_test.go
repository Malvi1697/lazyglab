package gitlab

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gogitlab "gitlab.com/gitlab-org/api/client-go"
)

// setupTestClient creates an httptest server with the given handler and returns
// a Client pointing at that server. The caller must call srv.Close() when done.
//
//nolint:unused // test helper prepared for upcoming gitlab package tests
func setupTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	apiClient, err := gogitlab.NewClient("test-token", gogitlab.WithBaseURL(srv.URL+"/api/v4"))
	if err != nil {
		srv.Close()
		t.Fatalf("creating test gitlab client: %v", err)
	}
	return &Client{api: apiClient, hostname: "test.gitlab.com"}, srv
}
