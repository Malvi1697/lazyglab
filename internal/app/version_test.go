package app

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"0.2.0", "0.1.0", true},
		{"0.1.0", "0.2.0", false},
		{"0.1.0", "0.1.0", false},
		{"1.0.0", "0.9.9", true},
		{"0.10.0", "0.9.0", true},
		{"0.1.1", "0.1.0", true},
		{"0.1.0", "0.1.1", false},
		{"1.0.0", "0.0.1", true},
		{"0.2", "0.1.0", true},
		{"0.1.0", "0.2", false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_vs_%s", tt.a, tt.b), func(t *testing.T) {
			got := isNewer(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("isNewer(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCheckForUpdate_NewerVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name": "v0.2.0"}`)
	}))
	defer server.Close()

	msg := checkForUpdateFrom(server.URL, "0.1.0")
	if msg == "" {
		t.Fatal("expected update message, got empty")
	}
	if !strings.Contains(msg, "v0.1.0") || !strings.Contains(msg, "v0.2.0") {
		t.Errorf("unexpected message: %s", msg)
	}
}

func TestCheckForUpdate_SameVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name": "v0.1.0"}`)
	}))
	defer server.Close()

	msg := checkForUpdateFrom(server.URL, "0.1.0")
	if msg != "" {
		t.Errorf("expected no message for same version, got: %s", msg)
	}
}

func TestCheckForUpdate_OlderRemote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name": "v0.0.9"}`)
	}))
	defer server.Close()

	msg := checkForUpdateFrom(server.URL, "0.1.0")
	if msg != "" {
		t.Errorf("expected no message when remote is older, got: %s", msg)
	}
}

func TestCheckForUpdate_DevSuffix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name": "v0.2.0"}`)
	}))
	defer server.Close()

	msg := checkForUpdateFrom(server.URL, "0.1.0-dev")
	if msg == "" {
		t.Fatal("expected update message for dev version, got empty")
	}
	if !strings.Contains(msg, "v0.1.0") {
		t.Errorf("dev suffix should be stripped, got: %s", msg)
	}
}

func TestCheckForUpdate_DevSuffixSameVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name": "v0.1.0"}`)
	}))
	defer server.Close()

	msg := checkForUpdateFrom(server.URL, "0.1.0-dev")
	if msg != "" {
		t.Errorf("expected no message for dev of same version, got: %s", msg)
	}
}

func TestCheckForUpdate_VPrefix(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name": "v0.2.0"}`)
	}))
	defer server.Close()

	msg := checkForUpdateFrom(server.URL, "v0.1.0")
	if msg == "" {
		t.Fatal("expected update message with v-prefixed current version")
	}
}

func TestCheckForUpdate_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	msg := checkForUpdateFrom(server.URL, "0.1.0")
	if msg != "" {
		t.Errorf("expected no message on server error, got: %s", msg)
	}
}

func TestCheckForUpdate_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `not json`)
	}))
	defer server.Close()

	msg := checkForUpdateFrom(server.URL, "0.1.0")
	if msg != "" {
		t.Errorf("expected no message on invalid JSON, got: %s", msg)
	}
}

func TestCheckForUpdate_EmptyTagName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"tag_name": ""}`)
	}))
	defer server.Close()

	msg := checkForUpdateFrom(server.URL, "0.1.0")
	if msg != "" {
		t.Errorf("expected no message on empty tag, got: %s", msg)
	}
}

func TestCheckForUpdate_Unreachable(t *testing.T) {
	msg := checkForUpdateFrom("http://192.0.2.1:1", "0.1.0")
	if msg != "" {
		t.Errorf("expected no message on unreachable server, got: %s", msg)
	}
}
