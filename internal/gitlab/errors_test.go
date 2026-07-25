package gitlab

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	gogitlab "gitlab.com/gitlab-org/api/client-go"
)

// apiErr builds an *ErrorResponse the way the API client would.
func apiErr(status int, message string, body string) error {
	return &gogitlab.ErrorResponse{
		Response: &http.Response{StatusCode: status},
		Message:  message,
		Body:     []byte(body),
	}
}

func TestIsAuthError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil is not an auth error", nil, false},
		{"401 always counts", apiErr(http.StatusUnauthorized, "", ""), true},
		{
			"revoked token (the real-world case)",
			apiErr(http.StatusUnauthorized, "", `{"error":"invalid_token","error_description":"Token was revoked."}`),
			true,
		},
		{
			"403 on a project is a permission problem, not an auth one",
			apiErr(http.StatusForbidden, "403 Forbidden", ""),
			false,
		},
		{
			"403 blaming the token's scope counts",
			apiErr(http.StatusForbidden, "insufficient_scope", ""),
			true,
		},
		{"404 never counts", apiErr(http.StatusNotFound, "404 Project Not Found", ""), false},
		{"500 never counts", apiErr(http.StatusInternalServerError, "", ""), false},
		{
			"our own ValidateToken failure counts",
			errors.New("authentication failed: invalid or expired token"),
			true,
		},
		{"an unrelated error does not", errors.New("connection refused"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsAuthError(tc.err); got != tc.want {
				t.Errorf("IsAuthError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsAuthError_WrappedError(t *testing.T) {
	// Callers wrap API errors with context; detection must survive that.
	wrapped := fmt.Errorf("listing projects: %w", apiErr(http.StatusUnauthorized, "", ""))
	if !IsAuthError(wrapped) {
		t.Error("expected a wrapped 401 to be detected as an auth error")
	}
}
