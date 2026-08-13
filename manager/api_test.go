package manager

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInfo(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/info", nil)
	service, _ := New("test", Config{}, nil)
	service.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestAdminRequiresManagerKey(t *testing.T) {
	service, _ := New("test", Config{AdminAPIKey: "manager-secret"}, nil)
	recorder := httptest.NewRecorder()
	service.router().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestAdminResourceProxyUsesInternalKey(t *testing.T) {
	resources := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/internal/google/accounts" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("X-Admin-API-Key") != "internal-secret" {
			t.Errorf("internal key was not forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"items":[]}`)
	}))
	defer resources.Close()
	service, _ := New("test", Config{AdminAPIKey: "manager-secret", Resources: ResourcesConfig{BaseURL: resources.URL, AdminAPIKey: "internal-secret", Timeout: time.Second}}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/admin/google/accounts", nil)
	request.Header.Set("Authorization", "Bearer manager-secret")
	service.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"items"`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestGatewayProxyForwardsGatewayKey(t *testing.T) {
	resources := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer gw_sk_test" {
			t.Errorf("gateway key was not forwarded")
		}
		_, _ = io.WriteString(w, `{"browser":{"id":"brw_test"}}`)
	}))
	defer resources.Close()
	service, _ := New("test", Config{Resources: ResourcesConfig{BaseURL: resources.URL, Timeout: time.Second}}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/browser", nil)
	request.Header.Set("Authorization", "Bearer gw_sk_test")
	service.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSharedAdminFrontend(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	service, _ := New("test", Config{}, nil)
	service.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
}
