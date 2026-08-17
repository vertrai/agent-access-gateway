package manager

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"rsc.io/qr"
)

const weixinAttemptLifetime = 10 * time.Minute

type WeixinCredentials struct {
	AccountID string
	Token     string
	BaseURL   string
	UserID    string
}

type weixinAttempt struct {
	ID, UserID, PollSecret, QRContent, ProviderBase string
	ExpiresAt, CredentialExpiresAt                  time.Time
	Credentials                                     *WeixinCredentials
	Polling                                         bool
}

func (m *Manager) startWeixinOnboarding(c *gin.Context) {
	var input struct {
		UserID string `json:"userId"`
	}
	if c.ShouldBindJSON(&input) != nil || strings.TrimSpace(input.UserID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userId is required"})
		return
	}
	var payload struct {
		QRCode string `json:"qrcode"`
		Image  string `json:"qrcode_img_content"`
	}
	if err := m.weixinGET(c.Request.Context(), m.weixinBaseURL+"/ilink/bot/get_bot_qrcode?bot_type=3", &payload); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if payload.QRCode == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "iLink QR response is incomplete"})
		return
	}
	content := payload.Image
	if content == "" {
		content = payload.QRCode
	}
	code, err := qr.Encode(content, qr.M)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "encode iLink QR code: " + err.Error()})
		return
	}
	qrDataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(code.PNG())
	attempt := weixinAttempt{ID: uuid.NewString(), UserID: strings.TrimSpace(input.UserID), PollSecret: payload.QRCode, QRContent: content, ProviderBase: m.weixinBaseURL, ExpiresAt: time.Now().UTC().Add(weixinAttemptLifetime)}
	m.weixinMu.Lock()
	for id, old := range m.weixinAttempts {
		deadline := old.ExpiresAt
		if old.Credentials != nil {
			deadline = old.CredentialExpiresAt
		}
		if old.UserID == attempt.UserID || time.Now().UTC().After(deadline) {
			delete(m.weixinAttempts, id)
		}
	}
	m.weixinAttempts[attempt.ID] = attempt
	m.weixinMu.Unlock()
	c.JSON(http.StatusCreated, gin.H{"attemptId": attempt.ID, "qrImage": qrDataURL, "expiresAt": attempt.ExpiresAt, "intervalSeconds": 2})
}

func (m *Manager) pollWeixinOnboarding(c *gin.Context) {
	m.weixinMu.Lock()
	attempt, ok := m.weixinAttempts[c.Param("attempt")]
	if ok && attempt.Polling {
		m.weixinMu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "Weixin onboarding poll is already in progress"})
		return
	}
	if ok {
		attempt.Polling = true
		m.weixinAttempts[attempt.ID] = attempt
	}
	m.weixinMu.Unlock()
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Weixin onboarding attempt not found"})
		return
	}
	defer func() {
		m.weixinMu.Lock()
		if current, exists := m.weixinAttempts[attempt.ID]; exists {
			current.Polling = false
			m.weixinAttempts[attempt.ID] = current
		}
		m.weixinMu.Unlock()
	}()
	if attempt.Credentials != nil {
		if time.Now().UTC().After(attempt.CredentialExpiresAt) {
			m.consumeWeixinAttempt(attempt.ID)
			c.JSON(http.StatusGone, gin.H{"state": "expired"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"state": "connected", "accountId": attempt.Credentials.AccountID, "userId": attempt.Credentials.UserID})
		return
	}
	if time.Now().UTC().After(attempt.ExpiresAt) {
		m.consumeWeixinAttempt(attempt.ID)
		c.JSON(http.StatusGone, gin.H{"state": "expired"})
		return
	}
	var payload map[string]any
	endpoint := strings.TrimRight(attempt.ProviderBase, "/") + "/ilink/bot/get_qrcode_status?qrcode=" + url.QueryEscape(attempt.PollSecret)
	if err := m.weixinGET(c.Request.Context(), endpoint, &payload); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	status, _ := payload["status"].(string)
	if status == "scaned_but_redirect" {
		if host, _ := payload["redirect_host"].(string); host != "" {
			base, err := allowedWeixinURL("https://" + host)
			if err != nil {
				c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
				return
			}
			attempt.ProviderBase = base
		}
	}
	if status == "expired" {
		m.consumeWeixinAttempt(attempt.ID)
		c.JSON(http.StatusOK, gin.H{"state": "expired"})
		return
	}
	if status != "confirmed" {
		state := "waiting"
		if status == "scaned" || status == "scaned_but_redirect" {
			state = "scanned"
		}
		m.updateWeixinAttempt(attempt)
		c.JSON(http.StatusOK, gin.H{"state": state})
		return
	}
	credential := &WeixinCredentials{AccountID: stringField(payload, "ilink_bot_id"), Token: stringField(payload, "bot_token"), BaseURL: stringField(payload, "baseurl"), UserID: stringField(payload, "ilink_user_id")}
	if credential.BaseURL == "" {
		credential.BaseURL = m.weixinBaseURL
	}
	base, err := allowedWeixinURL(credential.BaseURL)
	if err != nil || credential.AccountID == "" || credential.Token == "" || credential.UserID == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "iLink credential response is incomplete or untrusted"})
		return
	}
	credential.BaseURL = base
	attempt.Credentials = credential
	attempt.CredentialExpiresAt = time.Now().UTC().Add(24 * time.Hour)
	m.updateWeixinAttempt(attempt)
	c.JSON(http.StatusOK, gin.H{"state": "connected", "accountId": credential.AccountID, "userId": credential.UserID})
}

func (m *Manager) cancelWeixinOnboarding(c *gin.Context) {
	m.consumeWeixinAttempt(c.Param("attempt"))
	c.Status(http.StatusNoContent)
}

func (m *Manager) getWeixinOnboardingCredentials(c *gin.Context) {
	m.weixinMu.Lock()
	defer m.weixinMu.Unlock()
	attempt, ok := m.weixinAttempts[c.Param("attempt")]
	if !ok || attempt.Credentials == nil || time.Now().UTC().After(attempt.CredentialExpiresAt) {
		c.JSON(http.StatusConflict, gin.H{"error": "Weixin binding is not confirmed or has expired"})
		return
	}
	credentials := attempt.Credentials
	for name, value := range map[string]string{"WEIXIN_ACCOUNT_ID": credentials.AccountID, "WEIXIN_TOKEN": credentials.Token, "WEIXIN_BASE_URL": credentials.BaseURL, "WEIXIN_ALLOWED_USERS": credentials.UserID} {
		if !safeDotenvValue(value) {
			c.JSON(http.StatusBadGateway, gin.H{"error": "iLink returned an unsafe value for " + name})
			return
		}
	}
	env := map[string]string{
		"WEIXIN_ACCOUNT_ID":    credentials.AccountID,
		"WEIXIN_TOKEN":         credentials.Token,
		"WEIXIN_BASE_URL":      credentials.BaseURL,
		"WEIXIN_DM_POLICY":     "allowlist",
		"WEIXIN_ALLOWED_USERS": credentials.UserID,
	}
	dotenv := fmt.Sprintf("WEIXIN_ACCOUNT_ID=%s\nWEIXIN_TOKEN=%s\nWEIXIN_BASE_URL=%s\nWEIXIN_DM_POLICY=allowlist\nWEIXIN_ALLOWED_USERS=%s\n", credentials.AccountID, credentials.Token, credentials.BaseURL, credentials.UserID)
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{"env": env, "dotenv": dotenv})
}

func safeDotenvValue(value string) bool {
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	return !strings.ContainsAny(value, "\r\n\x00#\"'")
}

func (m *Manager) updateWeixinAttempt(attempt weixinAttempt) {
	m.weixinMu.Lock()
	if current, ok := m.weixinAttempts[attempt.ID]; ok && current.Credentials == nil {
		attempt.Polling = current.Polling
		m.weixinAttempts[attempt.ID] = attempt
	}
	m.weixinMu.Unlock()
}
func (m *Manager) consumeWeixinAttempt(id string) {
	if id == "" {
		return
	}
	m.weixinMu.Lock()
	delete(m.weixinAttempts, id)
	m.weixinMu.Unlock()
}
func stringField(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return value
}

func (m *Manager) weixinGET(ctx context.Context, endpoint string, output any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("iLink-App-Id", "bot")
	req.Header.Set("iLink-App-ClientVersion", "131584")
	res, err := m.weixinClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("iLink returned HTTP %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(output)
}

func allowedWeixinURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Port() != "" || parsed.Hostname() == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("invalid iLink base URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "weixin.qq.com" && !strings.HasSuffix(host, ".weixin.qq.com") {
		return "", fmt.Errorf("untrusted iLink host")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}
