package gitlab

import (
	"errors"
	"net/http"
	"strings"

	gogitlab "gitlab.com/gitlab-org/api/client-go"
)

// IsAuthError reports whether err means the stored token itself is unusable —
// missing, expired, revoked or lacking the required scope — so the user has to
// re-authenticate. A per-project permission problem is deliberately not an auth
// error: it must not trigger a re-authentication prompt.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}

	var resp *gogitlab.ErrorResponse
	if errors.As(err, &resp) {
		if resp.Response == nil {
			return blamesToken(resp.Message)
		}
		switch resp.Response.StatusCode {
		case http.StatusUnauthorized:
			return true
		case http.StatusForbidden:
			// 403 is usually "you may not touch this project"; treat it as an auth
			// failure only when GitLab blames the token itself (e.g. missing scope).
			return blamesToken(resp.Message) || blamesToken(string(resp.Body))
		default:
			return false
		}
	}

	// Errors raised outside the API client, e.g. our own ValidateToken.
	return blamesToken(err.Error())
}

// blamesToken reports whether a GitLab error message points at the token.
func blamesToken(msg string) bool {
	msg = strings.ToLower(msg)
	for _, needle := range []string{
		"invalid_token",
		"invalid or expired token",
		"insufficient_scope",
		"authentication failed",
		"revoked",
		"unauthorized",
	} {
		if strings.Contains(msg, needle) {
			return true
		}
	}
	return false
}
