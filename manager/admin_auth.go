package manager

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const adminSessionCookie = "hub_manager_admin_session"

type adminIdentity struct {
	Email, Name, Picture string
}

type adminIdentityProvider interface {
	AuthorizationURL(state, verifier string) string
	Exchange(context.Context, string, string) (adminIdentity, error)
}

type googleAdminIdentityProvider struct {
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func newGoogleAdminIdentityProvider(config AdminGoogleConfig) *googleAdminIdentityProvider {
	return &googleAdminIdentityProvider{
		oauth:    oauth2.Config{ClientID: config.ClientID, ClientSecret: config.ClientSecret, RedirectURL: config.RedirectURL, Endpoint: google.Endpoint, Scopes: []string{oidc.ScopeOpenID, "email", "profile"}},
		verifier: oidc.NewVerifier("https://accounts.google.com", oidc.NewRemoteKeySet(context.Background(), "https://www.googleapis.com/oauth2/v3/certs"), &oidc.Config{ClientID: config.ClientID}),
	}
}

func (p *googleAdminIdentityProvider) AuthorizationURL(state, verifier string) string {
	return p.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("prompt", "select_account"))
}

func (p *googleAdminIdentityProvider) Exchange(ctx context.Context, code, verifier string) (adminIdentity, error) {
	token, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return adminIdentity{}, fmt.Errorf("exchange Google authorization code: %w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok {
		return adminIdentity{}, errors.New("Google response did not include an ID token")
	}
	idToken, err := p.verifier.Verify(ctx, raw)
	if err != nil {
		return adminIdentity{}, fmt.Errorf("verify Google ID token: %w", err)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil || !claims.EmailVerified || strings.TrimSpace(claims.Email) == "" {
		return adminIdentity{}, errors.New("Google identity must contain a verified email")
	}
	return adminIdentity{Email: strings.ToLower(strings.TrimSpace(claims.Email)), Name: claims.Name, Picture: claims.Picture}, nil
}

type adminSession struct {
	Identity  adminIdentity
	ExpiresAt int64  `json:"expiresAt"`
	Nonce     string `json:"nonce"`
}

type adminAuthenticator struct {
	provider adminIdentityProvider
	allowed  map[string]struct{}
	secure   bool
	lifetime time.Duration
	secret   []byte
}

func newAdminAuthenticator(config AdminGoogleConfig) (*adminAuthenticator, error) {
	allowed := make(map[string]struct{}, len(config.AllowedEmails))
	for _, email := range config.AllowedEmails {
		if normalized := strings.ToLower(strings.TrimSpace(email)); normalized != "" {
			allowed[normalized] = struct{}{}
		}
	}
	if config.SessionLifetime <= 0 {
		config.SessionLifetime = 12 * time.Hour
	}
	var provider adminIdentityProvider
	configured := config.ClientID != "" || config.ClientSecret != "" || config.RedirectURL != "" || len(allowed) > 0 || config.SessionSecret != ""
	if configured {
		if config.ClientID == "" || config.ClientSecret == "" || config.RedirectURL == "" || len(allowed) == 0 || len(config.SessionSecret) < 32 {
			return nil, errors.New("admin Google clientID, clientSecret, redirectURL, 32-character sessionSecret and at least one allowed email are required")
		}
		provider = newGoogleAdminIdentityProvider(config)
	}
	return &adminAuthenticator{provider: provider, allowed: allowed, secure: config.CookieSecure, lifetime: config.SessionLifetime, secret: []byte(config.SessionSecret)}, nil
}

func (a *adminAuthenticator) issueSession(identity adminIdentity) (string, error) {
	nonce, err := randomAdminToken()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(adminSession{Identity: identity, ExpiresAt: time.Now().Add(a.lifetime).Unix(), Nonce: nonce})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (a *adminAuthenticator) verifySession(token string) (adminIdentity, bool) {
	encoded, signature, ok := strings.Cut(token, ".")
	if !ok {
		return adminIdentity{}, false
	}
	provided, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return adminIdentity{}, false
	}
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(encoded))
	if !hmac.Equal(provided, mac.Sum(nil)) {
		return adminIdentity{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return adminIdentity{}, false
	}
	var session adminSession
	if json.Unmarshal(payload, &session) != nil || session.Identity.Email == "" || time.Now().Unix() >= session.ExpiresAt {
		return adminIdentity{}, false
	}
	return session.Identity, true
}

func randomAdminToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func adminCookie(name, value, path string, secure bool, maxAge int) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: path, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge}
}

func (m *Manager) adminLogin(c *gin.Context) {
	if m.adminAuth.provider == nil {
		c.Redirect(http.StatusFound, "/admin/login?error=not_configured")
		return
	}
	state, err := randomAdminToken()
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	verifier := oauth2.GenerateVerifier()
	http.SetCookie(c.Writer, adminCookie("admin_oauth_state", state, "/auth/google/callback", m.adminAuth.secure, 600))
	http.SetCookie(c.Writer, adminCookie("admin_oauth_verifier", verifier, "/auth/google/callback", m.adminAuth.secure, 600))
	c.Redirect(http.StatusFound, m.adminAuth.provider.AuthorizationURL(state, verifier))
}

func (m *Manager) adminCallback(c *gin.Context) {
	if m.adminAuth.provider == nil {
		c.Redirect(http.StatusFound, "/admin/login?error=not_configured")
		return
	}
	state, e1 := c.Request.Cookie("admin_oauth_state")
	verifier, e2 := c.Request.Cookie("admin_oauth_verifier")
	if e1 != nil || e2 != nil || state.Value == "" || state.Value != c.Query("state") {
		c.Redirect(http.StatusFound, "/admin/login?error=invalid_state")
		return
	}
	identity, err := m.adminAuth.provider.Exchange(c.Request.Context(), c.Query("code"), verifier.Value)
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/login?error=oauth_failed")
		return
	}
	if _, ok := m.adminAuth.allowed[identity.Email]; !ok {
		c.Redirect(http.StatusFound, "/admin/login?error=not_allowed")
		return
	}
	token, err := m.adminAuth.issueSession(identity)
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	http.SetCookie(c.Writer, adminCookie(adminSessionCookie, token, "/", m.adminAuth.secure, int(m.adminAuth.lifetime.Seconds())))
	http.SetCookie(c.Writer, adminCookie("admin_oauth_state", "", "/auth/google/callback", m.adminAuth.secure, -1))
	http.SetCookie(c.Writer, adminCookie("admin_oauth_verifier", "", "/auth/google/callback", m.adminAuth.secure, -1))
	c.Redirect(http.StatusFound, "/admin")
}

func (m *Manager) currentAdmin(c *gin.Context) (adminIdentity, bool) {
	cookie, err := c.Request.Cookie(adminSessionCookie)
	if err != nil || cookie.Value == "" {
		return adminIdentity{}, false
	}
	return m.adminAuth.verifySession(cookie.Value)
}

func (m *Manager) requireAdminPage(c *gin.Context) {
	if _, ok := m.currentAdmin(c); !ok {
		c.Redirect(http.StatusFound, "/admin/login")
		c.Abort()
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Next()
}

func (m *Manager) requireAdmin(c *gin.Context) {
	identity, ok := m.currentAdmin(c)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Google administrator login is required"})
		return
	}
	c.Set("adminEmail", identity.Email)
	c.Header("Cache-Control", "no-store")
	c.Next()
}

func (m *Manager) adminMe(c *gin.Context) {
	identity, ok := m.currentAdmin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"email": identity.Email, "name": identity.Name, "picture": identity.Picture})
}

func (m *Manager) adminLogout(c *gin.Context) {
	http.SetCookie(c.Writer, adminCookie(adminSessionCookie, "", "/", m.adminAuth.secure, -1))
	c.Redirect(http.StatusFound, "/admin/login")
}
