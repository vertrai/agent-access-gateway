package manager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var telegramAPIHTTPClient = &http.Client{Timeout: 10 * time.Second}

func (m *Manager) resolveTelegramBotLink(c *gin.Context) {
	var request struct {
		BotToken string `json:"botToken"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.BotToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "botToken is required"})
		return
	}
	token := strings.TrimSpace(request.BotToken)
	if strings.ContainsAny(token, "\r\n/ ?#") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Telegram bot token"})
		return
	}
	apiRequest, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, "https://api.telegram.org/bot"+token+"/getMe", nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Telegram bot token"})
		return
	}
	response, err := telegramAPIHTTPClient.Do(apiRequest)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Telegram getMe request failed"})
		return
	}
	defer response.Body.Close()
	var result struct {
		OK     bool `json:"ok"`
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil || response.StatusCode/100 != 2 || !result.OK || strings.TrimSpace(result.Result.Username) == "" {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Telegram did not recognize this bot token"})
		return
	}
	username := strings.TrimPrefix(strings.TrimSpace(result.Result.Username), "@")
	c.JSON(http.StatusOK, gin.H{"username": username, "botLink": telegramBotLink(username)})
}

func telegramBotLink(username string) string {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	if username == "" {
		return ""
	}
	return fmt.Sprintf("https://t.me/%s", url.PathEscape(username))
}
