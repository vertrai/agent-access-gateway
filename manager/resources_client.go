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

func (c *ResourcesClient) createAccessKey(ctx context.Context, ownerUserID string) (CreatedAccessKey, int, error) {
	raw, status, err := c.do(ctx, http.MethodPost, "/v1/internal/access-keys", map[string]string{"ownerUserId": ownerUserID}, "")
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
	ID          string `json:"id"`
	OwnerUserID string `json:"ownerUserId"`
	KeyPrefix   string `json:"keyPrefix"`
	Status      string `json:"status"`
	GoogleEmail string `json:"googleEmail,omitempty"`
	BrowserID   string `json:"browserId,omitempty"`
	TelegramBot string `json:"telegramBot,omitempty"`
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
func (c *ResourcesClient) telegramBot(ctx context.Context, key string) (string, error) {
	raw, status, err := c.do(ctx, http.MethodGet, "/v1/telegram-bot", nil, key)
	if err != nil {
		return "", err
	}
	if status/100 != 2 {
		return "", fmt.Errorf("resources telegram bot: HTTP %d: %s", status, raw)
	}
	var result struct {
		TelegramBot struct {
			BotToken string `json:"botToken"`
		} `json:"telegramBot"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", err
	}
	return result.TelegramBot.BotToken, nil
}
