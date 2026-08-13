package resouces

import (
	"net/http"
	"net/http/httptest"
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

func TestAdminPageIsNotMounted(t *testing.T) {
	g := New("test", Config{}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	g.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestTelegramAdminPage(t *testing.T) {
	g := New("test", Config{}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/telegram", nil)
	g.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("telegram admin page is unavailable")
	}
}

func TestAdminTestPage(t *testing.T) {
	g := New("test", Config{}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	g.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestAdminRequiresKey(t *testing.T) {
	g := New("test", Config{AdminAPIKey: "secret"}, nil)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/internal/access-keys", nil)
	g.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestTelegramAdminRoutesRequireKey(t *testing.T) {
	g := New("test", Config{AdminAPIKey: "secret"}, nil)
	for _, path := range []string{"/v1/internal/telegram/auth/init", "/v1/internal/telegram/bots/create"} {
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
