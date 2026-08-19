package manager

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type adminIdentityProviderStub struct{ identity adminIdentity }

func (s adminIdentityProviderStub) AuthorizationURL(_, _ string) string {
	return "https://accounts.example/auth"
}
func (s adminIdentityProviderStub) Exchange(context.Context, string, string) (adminIdentity, error) {
	return s.identity, nil
}

func TestAdminOAuthCallbackAllowsConfiguredEmail(t *testing.T) {
	service, _ := New("test", Config{}, nil)
	service.adminAuth.provider = adminIdentityProviderStub{identity: adminIdentity{Email: "admin@example.com"}}
	service.adminAuth.allowed["admin@example.com"] = struct{}{}

	request := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=state&code=code", nil)
	request.AddCookie(&http.Cookie{Name: "admin_oauth_state", Value: "state"})
	request.AddCookie(&http.Cookie{Name: "admin_oauth_verifier", Value: "verifier"})
	recorder := httptest.NewRecorder()
	service.router().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusFound || recorder.Header().Get("Location") != "/admin" {
		t.Fatalf("status=%d location=%q", recorder.Code, recorder.Header().Get("Location"))
	}
	foundSession := false
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == adminSessionCookie && cookie.Value != "" && cookie.HttpOnly {
			foundSession = true
		}
	}
	if !foundSession {
		t.Fatal("callback did not issue an HttpOnly administrator session")
	}
}

func TestAdminOAuthCallbackRejectsEmailOutsideAllowlist(t *testing.T) {
	service, _ := New("test", Config{}, nil)
	service.adminAuth.provider = adminIdentityProviderStub{identity: adminIdentity{Email: "outsider@example.com"}}
	service.adminAuth.allowed["admin@example.com"] = struct{}{}
	request := httptest.NewRequest(http.MethodGet, "/auth/google/callback?state=state&code=code", nil)
	request.AddCookie(&http.Cookie{Name: "admin_oauth_state", Value: "state"})
	request.AddCookie(&http.Cookie{Name: "admin_oauth_verifier", Value: "verifier"})
	recorder := httptest.NewRecorder()
	service.router().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusFound || recorder.Header().Get("Location") != "/admin/login?error=not_allowed" {
		t.Fatalf("status=%d location=%q", recorder.Code, recorder.Header().Get("Location"))
	}
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == adminSessionCookie && cookie.Value != "" {
			t.Fatal("rejected identity received a session")
		}
	}
}

func TestAdminGoogleConfigurationMustBeComplete(t *testing.T) {
	_, err := New("test", Config{AdminGoogle: AdminGoogleConfig{ClientID: "client"}}, nil)
	if err == nil {
		t.Fatal("expected incomplete Google administrator configuration to fail")
	}
}

func TestAdminSessionRejectsTampering(t *testing.T) {
	auth, err := newAdminAuthenticator(AdminGoogleConfig{})
	if err != nil {
		t.Fatal(err)
	}
	token, err := auth.issueSession(adminIdentity{Email: "admin@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if identity, ok := auth.verifySession(token); !ok || identity.Email != "admin@example.com" {
		t.Fatal("valid signed session was rejected")
	}
	if _, ok := auth.verifySession(token + "x"); ok {
		t.Fatal("tampered session was accepted")
	}
}
