package resouces

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func (g *Resouces) createTelegramBots(c *gin.Context) {
	var req struct {
		Count int `json:"count"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Count <= 0 {
		req.Count = 1
	}
	if remaining := g.telegram.CreateCooldownRemaining(); remaining > 0 {
		seconds := int((remaining + time.Second - 1) / time.Second)
		c.Header("Retry-After", strconv.Itoa(seconds))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Telegram BotFather rate limit cooldown is active", "retryAfterSeconds": seconds})
		return
	}
	ctx, cancel := withTelegramCreationTimeout(c, req.Count)
	defer cancel()
	created, err := g.telegram.CreateBots(ctx, req.Count)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error(), "created": created})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"created": created, "createdCount": len(created)})
}

func (g *Resouces) initTelegramAuth(c *gin.Context) {
	var req struct {
		Phone   string `json:"phone"`
		APIID   string `json:"apiId"`
		APIHash string `json:"apiHash"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), g.telegram.RequestTimeout())
	defer cancel()
	result, err := g.telegram.InitAuth(ctx, req.Phone, req.APIID, req.APIHash)
	if err != nil {
		if ctx.Err() != nil {
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "连接 Telegram 超时，请检查服务端到 Telegram 数据中心的网络连接。"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (g *Resouces) verifyTelegramAuth(c *gin.Context) {
	var req struct {
		AccountID string `json:"accountId"`
		Code      string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.AccountID) == "" || strings.TrimSpace(req.Code) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "accountId and code are required"})
		return
	}
	need2FA, err := g.telegram.VerifyCode(c.Request.Context(), req.AccountID, req.Code)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if !need2FA {
		go g.telegram.EnsureBotTokens(context.Background())
	}
	c.JSON(http.StatusOK, gin.H{"need2FA": need2FA})
}

func (g *Resouces) submitTelegram2FA(c *gin.Context) {
	var req struct {
		AccountID string `json:"accountId"`
		Password  string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.AccountID) == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "accountId and password are required"})
		return
	}
	if err := g.telegram.Submit2FA(c.Request.Context(), req.AccountID, req.Password); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	go g.telegram.EnsureBotTokens(context.Background())
	c.JSON(http.StatusOK, gin.H{"authorized": true})
}

func (g *Resouces) telegramAuthStatus(c *gin.Context) {
	result, err := g.telegram.Status(c.Request.Context(), c.Query("accountId"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func (g *Resouces) listTelegramAccounts(c *gin.Context) {
	rows, err := g.telegram.ListAccounts()
	if err != nil {
		g.internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func withTelegramCreationTimeout(c *gin.Context, count int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.Request.Context(), time.Duration(count)*5*time.Minute)
}
