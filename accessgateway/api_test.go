package accessgateway

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
	if !strings.Contains(body, "新增 Gateway API Key") || strings.Contains(body, "生成 / 轮换") {
		t.Fatalf("admin page does not describe additive access key creation")
	}
	if !strings.Contains(body, "assignedAccessKeyId") {
		t.Fatalf("admin page does not display google account access key ownership")
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
