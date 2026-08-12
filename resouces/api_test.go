package resouces

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInfo(t *testing.T) {
	g := New("test", Config{}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/info", nil)
	g.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestAdminPage(t *testing.T) {
	g := New("test", Config{}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	g.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("content type = %q", contentType)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "资源总览") || !strings.Contains(body, "/admin/telegram") {
		t.Fatalf("admin page does not expose the resource console navigation")
	}
}

func TestTelegramAdminPage(t *testing.T) {
	g := New("test", Config{}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/telegram", nil)
	g.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "BotFather") {
		t.Fatalf("telegram admin page is unavailable")
	}
}

func TestAdminTestPage(t *testing.T) {
	g := New("test", Config{}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	g.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("content type = %q", contentType)
	}
}

func TestAdminRequiresKey(t *testing.T) {
	g := New("test", Config{AdminAPIKey: "secret"}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/admin/users", nil)
	g.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestTelegramAdminRoutesRequireKey(t *testing.T) {
	g := New("test", Config{AdminAPIKey: "secret"}, nil)
	for _, path := range []string{"/v1/admin/telegram/auth/init", "/v1/admin/telegram/bots/create"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, path, nil)
		g.router().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("POST %s status = %d", path, recorder.Code)
		}
	}
}

func TestUserRoutesRequireGatewayAPIKey(t *testing.T) {
	g := New("test", Config{}, nil)
	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/v1/google-user"},
		{method: http.MethodGet, path: "/v1/google-user/access-token"},
		{method: http.MethodGet, path: "/v1/browser"},
		{method: http.MethodPost, path: "/v1/browser/reset"},
		{method: http.MethodPost, path: "/v1/browser/close"},
		{method: http.MethodGet, path: "/v1/telegram-bot"},
	}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(test.method, test.path, nil)
		g.router().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d", test.method, test.path, recorder.Code)
		}
	}
}
