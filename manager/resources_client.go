package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type ResourcesClient struct {
	base        *url.URL
	adminAPIKey string
	http        *http.Client
}

func NewResourcesClient(config ResourcesConfig) *ResourcesClient {
	base, _ := url.Parse(strings.TrimRight(config.BaseURL, "/"))
	return &ResourcesClient{base: base, adminAPIKey: config.AdminAPIKey, http: &http.Client{Timeout: config.Timeout}}
}

func (c *ResourcesClient) configured() bool {
	return c != nil && c.base != nil && c.base.Scheme != "" && c.base.Host != ""
}
func (c *ResourcesClient) do(ctx context.Context, method, path string, body any, gatewayKey string) ([]byte, int, error) {
	if !c.configured() {
		return nil, 0, fmt.Errorf("resources baseURL is not configured")
	}
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base.String()+path, reader)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if gatewayKey != "" {
		req.Header.Set("Authorization", "Bearer "+gatewayKey)
	} else {
		req.Header.Set("X-Admin-API-Key", c.adminAPIKey)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	return raw, res.StatusCode, err
}

type CreatedAccessKey struct {
	AccessKey     ResourceAccessKey `json:"accessKey"`
	GatewayAPIKey string            `json:"gatewayApiKey"`
}

type ResourceScopes struct {
	AllowGoogle   bool `json:"allowGoogle"`
	AllowBrowser  bool `json:"allowBrowser"`
	AllowTelegram bool `json:"allowTelegram"`
}

func (c *ResourcesClient) createAccessKey(ctx context.Context, ownerUserID string, scopes ResourceScopes) (CreatedAccessKey, int, error) {
	body := struct {
		OwnerUserID string `json:"ownerUserId"`
		ResourceScopes
	}{OwnerUserID: ownerUserID, ResourceScopes: scopes}
	raw, status, err := c.do(ctx, http.MethodPost, "/v1/internal/access-keys", body, "")
	if err != nil {
		return CreatedAccessKey{}, status, err
	}
	if status/100 != 2 {
		return CreatedAccessKey{}, status, fmt.Errorf("resources create access key: HTTP %d: %s", status, raw)
	}
	var result CreatedAccessKey
	if err := json.Unmarshal(raw, &result); err != nil {
		return CreatedAccessKey{}, status, err
	}
	return result, status, nil
}

type ResourceAccessKey struct {
	ID            string `json:"id"`
	OwnerUserID   string `json:"ownerUserId"`
	KeyPrefix     string `json:"keyPrefix"`
	Status        string `json:"status"`
	AllowGoogle   bool   `json:"allowGoogle"`
	AllowBrowser  bool   `json:"allowBrowser"`
	AllowTelegram bool   `json:"allowTelegram"`
	GoogleEmail   string `json:"googleEmail,omitempty"`
	BrowserID     string `json:"browserId,omitempty"`
	TelegramBot   string `json:"telegramBot,omitempty"`
}

func (c *ResourcesClient) updateAccessKeyScopes(ctx context.Context, id string, scopes ResourceScopes) (ResourceAccessKey, int, error) {
	raw, status, err := c.do(ctx, http.MethodPatch, "/v1/internal/access-keys/"+url.PathEscape(id)+"/scopes", scopes, "")
	if err != nil {
		return ResourceAccessKey{}, status, err
	}
	if status/100 != 2 {
		return ResourceAccessKey{}, status, fmt.Errorf("resources update access key scopes: HTTP %d: %s", status, raw)
	}
	var result struct {
		AccessKey ResourceAccessKey `json:"accessKey"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return ResourceAccessKey{}, status, err
	}
	return result.AccessKey, status, nil
}

func (c *ResourcesClient) listAccessKeys(ctx context.Context) ([]ResourceAccessKey, error) {
	raw, status, err := c.do(ctx, http.MethodGet, "/v1/internal/access-keys", nil, "")
	if err != nil {
		return nil, err
	}
	if status/100 != 2 {
		return nil, fmt.Errorf("resources access keys: HTTP %d: %s", status, raw)
	}
	var result struct {
		Items []ResourceAccessKey `json:"items"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}

type ResourceTelegramBot struct {
	BotToken string `json:"botToken"`
	Username string `json:"username"`
}

type ResourcesHTTPError struct {
	StatusCode int
	Body       string
}

func (e *ResourcesHTTPError) Error() string {
	return fmt.Sprintf("resources request failed: HTTP %d: %s", e.StatusCode, e.Body)
}

func (c *ResourcesClient) telegramBotDetails(ctx context.Context, key string) (ResourceTelegramBot, error) {
	raw, status, err := c.do(ctx, http.MethodGet, "/v1/telegram-bot", nil, key)
	if err != nil {
		return ResourceTelegramBot{}, err
	}
	if status/100 != 2 {
		return ResourceTelegramBot{}, &ResourcesHTTPError{StatusCode: status, Body: string(raw)}
	}
	var result struct {
		TelegramBot ResourceTelegramBot `json:"telegramBot"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return ResourceTelegramBot{}, err
	}
	return result.TelegramBot, nil
}

func (c *ResourcesClient) telegramBot(ctx context.Context, key string) (string, error) {
	bot, err := c.telegramBotDetails(ctx, key)
	return bot.BotToken, err
}
