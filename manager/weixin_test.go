package manager

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWeixinOnboardingDoesNotExposePollSecret(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/get_bot_qrcode" || r.URL.Query().Get("bot_type") != "3" {
			t.Fatalf("unexpected provider request: %s", r.URL.String())
		}
		if r.Header.Get("iLink-App-Id") != "bot" {
			t.Fatal("missing iLink app header")
		}
		_, _ = w.Write([]byte(`{"qrcode":"secret-poll-token","qrcode_img_content":"https://weixin.qq.com/x/scan"}`))
	}))
	defer provider.Close()
	m, err := New("test", Config{AdminAPIKey: "admin", Resources: ResourcesConfig{Timeout: time.Second}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m.weixinBaseURL = provider.URL
	m.weixinClient = provider.Client()
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/weixin/onboarding", bytes.NewBufferString(`{"userId":"user-a"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer admin")
	res := httptest.NewRecorder()
	m.router().ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "secret-poll-token") {
		t.Fatalf("poll secret leaked: %s", res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "data:image/png;base64,") {
		t.Fatalf("QR image missing: %s", res.Body.String())
	}
}

func TestAllowedWeixinURLRejectsUntrustedHost(t *testing.T) {
	if _, err := allowedWeixinURL("https://attacker.example"); err == nil {
		t.Fatal("expected untrusted host rejection")
	}
	if got, err := allowedWeixinURL("https://ilinkai.weixin.qq.com"); err != nil || got != "https://ilinkai.weixin.qq.com" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestResolveWeixinCredentialsClaimsAttemptOnce(t *testing.T) {
	m, err := New("test", Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m.weixinAttempts["attempt"] = weixinAttempt{UserID: "user-a", Credentials: &WeixinCredentials{Token: "secret"}, CredentialExpiresAt: time.Now().Add(time.Hour)}
	if _, err := m.resolveWeixinCredentials("user-a", "attempt"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.resolveWeixinCredentials("user-a", "attempt"); err == nil {
		t.Fatal("expected claimed attempt to reject a second consumer")
	}
	m.releaseWeixinAttempt("attempt")
	if _, err := m.resolveWeixinCredentials("user-a", "attempt"); err != nil {
		t.Fatalf("released attempt cannot be retried: %v", err)
	}
}
