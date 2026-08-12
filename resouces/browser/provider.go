package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type BrowserConfig struct {
	ProxyCountryCode string `json:"proxyCountryCode"`
	TimeoutMinutes   int    `json:"timeoutMinutes"`
	ProfileName      string `json:"profileName"`
	ProfileID        string `json:"profileId"`
}
type BrowserSession struct {
	ID, CDPURL, LiveURL, Status, ProfileID string
	StartedAt, TimeoutAt                   *time.Time
}
type BrowserProvider interface {
	Start(context.Context, BrowserConfig) (BrowserSession, error)
	Get(context.Context, string) (BrowserSession, error)
	Stop(context.Context, string) error
}

type BrowserProviderError struct {
	StatusCode int
	Message    string
}

func (e *BrowserProviderError) Error() string { return e.Message }
func IsBrowserProviderNotFound(err error) bool {
	var providerErr *BrowserProviderError
	return errors.As(err, &providerErr) && providerErr.StatusCode == http.StatusNotFound
}

type browserUseProvider struct {
	apiKey, baseURL string
	client          *http.Client
}

func NewBrowserUseProvider(key, baseURL string) BrowserProvider {
	return &browserUseProvider{key, strings.TrimRight(baseURL, "/"), &http.Client{Timeout: 3 * time.Minute}}
}
func (p *browserUseProvider) Start(ctx context.Context, cfg BrowserConfig) (BrowserSession, error) {
	if p.apiKey == "" {
		return BrowserSession{}, fmt.Errorf("browser api key is required")
	}
	profileID, err := p.ensureProfile(ctx, cfg.ProfileID, cfg.ProfileName)
	if err != nil {
		return BrowserSession{}, err
	}
	session, err := p.start(ctx, cfg, profileID)
	if err == nil {
		return session, nil
	}
	if cfg.ProfileID == "" || !strings.Contains(strings.ToLower(err.Error()), "profile") {
		return BrowserSession{}, err
	}
	recoveredID, recoverErr := p.ensureProfile(ctx, "", cfg.ProfileName)
	if recoverErr != nil {
		return BrowserSession{}, fmt.Errorf("recover browser profile: %w", recoverErr)
	}
	return p.start(ctx, cfg, recoveredID)
}

func (p *browserUseProvider) start(ctx context.Context, cfg BrowserConfig, profileID string) (BrowserSession, error) {
	body := map[string]any{"timeout": cfg.TimeoutMinutes, "proxyCountryCode": cfg.ProxyCountryCode}
	if profileID != "" {
		body["profileId"] = profileID
	}
	var raw struct {
		ID        string     `json:"id"`
		CDPURL    string     `json:"cdpUrl"`
		LiveURL   string     `json:"liveUrl"`
		Status    string     `json:"status"`
		StartedAt *time.Time `json:"startedAt"`
		TimeoutAt *time.Time `json:"timeoutAt"`
	}
	if err := p.call(ctx, http.MethodPost, "/browsers", body, &raw); err != nil {
		return BrowserSession{}, err
	}
	return BrowserSession{ID: raw.ID, CDPURL: raw.CDPURL, LiveURL: raw.LiveURL, Status: raw.Status, ProfileID: profileID, StartedAt: raw.StartedAt, TimeoutAt: raw.TimeoutAt}, nil
}

func (p *browserUseProvider) Get(ctx context.Context, id string) (BrowserSession, error) {
	var raw struct {
		ID        string     `json:"id"`
		CDPURL    string     `json:"cdpUrl"`
		LiveURL   string     `json:"liveUrl"`
		Status    string     `json:"status"`
		StartedAt *time.Time `json:"startedAt"`
		TimeoutAt *time.Time `json:"timeoutAt"`
	}
	if err := p.call(ctx, http.MethodGet, "/browsers/"+id, nil, &raw); err != nil {
		return BrowserSession{}, err
	}
	return BrowserSession{ID: raw.ID, CDPURL: raw.CDPURL, LiveURL: raw.LiveURL, Status: raw.Status, StartedAt: raw.StartedAt, TimeoutAt: raw.TimeoutAt}, nil
}

func (p *browserUseProvider) ensureProfile(ctx context.Context, id, name string) (string, error) {
	if id != "" || name == "" {
		return id, nil
	}
	var listing struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := p.call(ctx, http.MethodGet, "/profiles?pageSize=100&pageNumber=1", nil, &listing); err != nil {
		return "", fmt.Errorf("list browser profiles: %w", err)
	}
	for _, profile := range listing.Items {
		if profile.Name == name {
			return profile.ID, nil
		}
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := p.call(ctx, http.MethodPost, "/profiles", map[string]any{"name": name}, &created); err != nil {
		return "", fmt.Errorf("create browser profile: %w", err)
	}
	return created.ID, nil
}
func (p *browserUseProvider) Stop(ctx context.Context, id string) error {
	return p.call(ctx, http.MethodPatch, "/browsers/"+id, map[string]any{"action": "stop"}, nil)
}
func (p *browserUseProvider) call(ctx context.Context, method, path string, body, out any) error {
	var b []byte
	var err error
	if body != nil {
		b, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("X-Browser-Use-API-Key", p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &BrowserProviderError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("browser provider returned %s: %s", resp.Status, strings.TrimSpace(string(data)))}
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}
