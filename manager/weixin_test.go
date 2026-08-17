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

func TestConfirmedWeixinCredentialsReturnHermesEnvironment(t *testing.T) {
	m, err := New("test", Config{AdminAPIKey: "admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m.weixinAttempts["attempt"] = weixinAttempt{Credentials: &WeixinCredentials{AccountID: "bot@im.bot", Token: "secret-token", BaseURL: "https://ilinkai.weixin.qq.com", UserID: "wx-user"}, CredentialExpiresAt: time.Now().Add(time.Hour)}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/weixin/onboarding/attempt/credentials", nil)
	req.Header.Set("Authorization", "Bearer admin")
	res := httptest.NewRecorder()
	m.router().ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	for _, expected := range []string{"WEIXIN_ACCOUNT_ID", "bot@im.bot", "WEIXIN_TOKEN", "secret-token", "WEIXIN_DM_POLICY=allowlist", "WEIXIN_ALLOWED_USERS=wx-user"} {
		if !strings.Contains(res.Body.String(), expected) {
			t.Errorf("response missing %q: %s", expected, res.Body.String())
		}
	}
	if res.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control=%q", res.Header().Get("Cache-Control"))
	}
}

func TestConfirmedWeixinCredentialsRejectDotenvInjection(t *testing.T) {
	m, err := New("test", Config{AdminAPIKey: "admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	m.weixinAttempts["attempt"] = weixinAttempt{Credentials: &WeixinCredentials{AccountID: "bot@im.bot", Token: "secret\nINJECTED=yes", BaseURL: "https://ilinkai.weixin.qq.com", UserID: "wx-user"}, CredentialExpiresAt: time.Now().Add(time.Hour)}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/weixin/onboarding/attempt/credentials", nil)
	req.Header.Set("Authorization", "Bearer admin")
	res := httptest.NewRecorder()
	m.router().ServeHTTP(res, req)
	if res.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
	}
	if strings.Contains(res.Body.String(), "INJECTED=yes") {
		t.Fatalf("unsafe value leaked: %s", res.Body.String())
	}
}
