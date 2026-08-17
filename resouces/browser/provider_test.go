package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBrowserProviderCreatesProfileAndSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/profiles":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/profiles":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "profile-1"})
		case r.Method == http.MethodPost && r.URL.Path == "/browsers":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["profileId"] != "profile-1" {
				t.Errorf("profileId = %v", body["profileId"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "browser-1", "cdpUrl": "ws://browser"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	provider := NewBrowserUseProvider("key", server.URL)
	session, err := provider.Start(context.Background(), BrowserConfig{ProfileName: "agent-profile", TimeoutMinutes: 10})
	if err != nil {
		t.Fatal(err)
	}
	if session.ID != "browser-1" || session.ProfileID != "profile-1" {
		t.Fatalf("session = %+v", session)
	}
}

func TestBrowserProviderGetsCurrentStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/browsers/browser-1" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "browser-1", "status": "active", "cdpUrl": "ws://current", "liveUrl": "https://live"})
	}))
	defer server.Close()
	provider := NewBrowserUseProvider("key", server.URL)
	session, err := provider.Get(context.Background(), "browser-1")
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != "active" || session.CDPURL != "ws://current" {
		t.Fatalf("session = %+v", session)
	}
}

func TestBrowserProviderReportsNotFound(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	provider := NewBrowserUseProvider("key", server.URL)
	_, err := provider.Get(context.Background(), "missing")
	if !IsBrowserProviderNotFound(err) {
		t.Fatalf("error = %v", err)
	}
}

func TestBrowserProviderStopsSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/browsers/browser-1" {
			http.NotFound(w, r)
			return
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["action"] != "stop" {
			t.Fatalf("action = %q", body["action"])
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	provider := NewBrowserUseProvider("key", server.URL)
	if err := provider.Stop(context.Background(), "browser-1"); err != nil {
		t.Fatal(err)
	}
}
