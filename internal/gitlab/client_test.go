package gitlab

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateToken_success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/user" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"username":"testuser"}`))
	}))
	defer srv.Close()

	username, err := ValidateToken(srv.URL, "test-token", srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if username != "testuser" {
		t.Errorf("want testuser, got %s", username)
	}
}

func TestValidateToken_unauthorized(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"401 Unauthorized"}`))
	}))
	defer srv.Close()

	_, err := ValidateToken(srv.URL, "bad-token", srv.Client())
	if err == nil {
		t.Fatal("expected error for bad token")
	}
}

func TestValidateToken_serverError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := ValidateToken(srv.URL, "token", srv.Client())
	if err == nil {
		t.Fatal("expected error for server error")
	}
}
