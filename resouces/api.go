package resouces

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/vertrai/hub/common"
	resourcebrowser "github.com/vertrai/hub/resouces/browser"
	resourcegoogle "github.com/vertrai/hub/resouces/google"
	"github.com/vertrai/hub/resouces/schema"
	resourcetelegram "github.com/vertrai/hub/resouces/telegram"
	"gorm.io/gorm"
)

const gatewayPrincipalContext = "gatewayPrincipal"

type gatewayPrincipal struct {
	AccessKey schema.AccessKey
}

func (g *Resouces) router() *gin.Engine {
	r := gin.New()
	r.Use(common.RequestLogger(log), gin.Recovery(), common.CORSMiddleware())
	r.GET("/info", g.info)
	internal := r.Group("/v1/internal", g.requireAdmin)
	internal.POST("/access-keys", g.createAccessKey)
	internal.GET("/access-keys", g.listAccessKeys)
	admin := internal
	admin.POST("/google/accounts", g.createGoogleAccount)
	admin.POST("/google/accounts/batch", g.createGoogleAccountsBatch)
	admin.GET("/google/accounts", g.listGoogleAccounts)
	admin.POST("/telegram/bots", g.importTelegramBot)
	admin.GET("/telegram/bots", g.listTelegramBots)
	admin.POST("/telegram/bots/create", g.createTelegramBots)
	admin.POST("/telegram/auth/init", g.initTelegramAuth)
	admin.POST("/telegram/auth/verify", g.verifyTelegramAuth)
	admin.POST("/telegram/auth/2fa", g.submitTelegram2FA)
	admin.GET("/telegram/auth/status", g.telegramAuthStatus)
	admin.GET("/telegram/auth/accounts", g.listTelegramAccounts)
	user := r.Group("/v1", g.requireGatewayAPIKey)
	user.GET("/google-user", g.getGoogleUser)
	user.GET("/google-user/access-token", g.issueGoogleToken)
	user.POST("/google-user/test/gmail/send", g.testSendGmail)
	user.POST("/google-user/test/drive/folders", g.testCreateDriveFolder)
	user.GET("/browser", g.currentBrowser)
	user.POST("/browser/reset", g.resetBrowser)
	user.POST("/browser/close", g.closeBrowser)
	user.GET("/telegram-bot", g.getTelegramBot)
	return r
}

func (g *Resouces) listAccessKeys(c *gin.Context) {
	type accessKeySummary struct {
		schema.AccessKey
		GoogleEmail string `json:"googleEmail,omitempty"`
		BrowserID   string `json:"browserId,omitempty"`
		TelegramBot string `json:"telegramBot,omitempty"`
	}
	var keys []schema.AccessKey
	if err := g.wdb.Db.Order("created_at desc").Find(&keys).Error; err != nil {
		g.internalError(c, err)
		return
	}
	items := make([]accessKeySummary, 0, len(keys))
	for _, key := range keys {
		keyItem := accessKeySummary{AccessKey: key}
		var account schema.GoogleAccount
		if err := g.wdb.Db.Where("assigned_access_key_id = ?", key.ID).First(&account).Error; err == nil {
			keyItem.GoogleEmail = account.Email
		}
		var browser schema.Browser
		if err := g.wdb.Db.Where("access_key_id = ?", key.ID).First(&browser).Error; err == nil {
			keyItem.BrowserID = browser.ID
		}
		var telegramBot schema.TelegramBot
		if err := g.wdb.Db.Where("assigned_access_key_id = ?", key.ID).First(&telegramBot).Error; err == nil {
			keyItem.TelegramBot = telegramBot.Username
		}
		items = append(items, keyItem)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (g *Resouces) runAPI(endpoint string) {
	g.apiServer = &http.Server{Addr: endpoint, Handler: g.router()}
	if err := g.apiServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("http listen", "err", err)
	}
}
func (g *Resouces) info(c *gin.Context) {
	c.JSON(200, gin.H{"name": "hub", "env": g.env})
}
func (g *Resouces) requireAdmin(c *gin.Context) {
	got := firstNonEmpty(c.GetHeader("X-Admin-API-Key"), bearer(c.GetHeader("Authorization")))
	if g.config.AdminAPIKey == "" || len(got) != len(g.config.AdminAPIKey) || subtle.ConstantTimeCompare([]byte(got), []byte(g.config.AdminAPIKey)) != 1 {
		c.AbortWithStatusJSON(401, gin.H{"error": "valid admin api key is required"})
	}
}

func (g *Resouces) createAccessKey(c *gin.Context) {
	var req struct {
		OwnerUserID string `json:"ownerUserId"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.OwnerUserID) == "" {
		c.JSON(400, gin.H{"error": "ownerUserId is required"})
		return
	}
	ownerUserID := strings.TrimSpace(req.OwnerUserID)
	key, err := newAccessKey()
	if err != nil {
		g.internalError(c, err)
		return
	}
	keyID, err := newID("key_")
	if err != nil {
		g.internalError(c, err)
		return
	}
	accessKey := schema.AccessKey{ID: keyID, OwnerUserID: ownerUserID, KeyHash: hashSecret(key), KeyPrefix: secretPrefix(key), Status: schema.StatusActive}
	err = g.wdb.Db.Create(&accessKey).Error
	if err != nil {
		g.internalError(c, err)
		return
	}
	c.JSON(200, gin.H{"accessKey": accessKey, "gatewayApiKey": key})
}

func (g *Resouces) createGoogleAccount(c *gin.Context) {
	var req struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" {
		c.JSON(400, gin.H{"error": "email is required"})
		return
	}
	if req.Password == "" {
		var err error
		req.Password, err = resourcegoogle.RandomPassword()
		if err != nil {
			g.internalError(c, err)
			return
		}
	}
	row, err := g.google.CreateAccount(c.Request.Context(), req.Email, req.Password, firstNonEmpty(req.GivenName, "Agent"), firstNonEmpty(req.FamilyName, "User"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, gin.H{"account": row})
}

func (g *Resouces) createGoogleAccountsBatch(c *gin.Context) {
	var req struct {
		Count  int    `json:"count"`
		Domain string `json:"domain"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Count <= 0 {
		req.Count = 1
	}
	if req.Count > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "count must be <= 100"})
		return
	}
	domain := firstNonEmpty(req.Domain, g.google.Domain())
	if !strings.EqualFold(domain, g.google.Domain()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain must match configured workspace domain"})
		return
	}
	created, err := g.google.CreateAccounts(c.Request.Context(), req.Count, domain)
	if err != nil {
		status := http.StatusBadGateway
		if len(created) > 0 {
			status = http.StatusMultiStatus
		}
		c.JSON(status, gin.H{"error": err.Error(), "created": created, "createdCount": len(created), "requestedCount": req.Count})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"created": created, "createdCount": len(created), "requestedCount": req.Count})
}
func (g *Resouces) listGoogleAccounts(c *gin.Context) {
	rows, err := g.google.ListAccounts()
	if err != nil {
		g.internalError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": rows})
}

func (g *Resouces) getGoogleUser(c *gin.Context) {
	principal := mustGatewayPrincipal(c)
	account, err := g.google.AcquireAccount(c.Request.Context(), principal.AccessKey.ID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"googleUser": account})
}
func (g *Resouces) issueGoogleToken(c *gin.Context) {
	principal := mustGatewayPrincipal(c)
	token, account, err := g.google.IssueToken(c.Request.Context(), principal.AccessKey.ID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"accessToken": token.AccessToken, "tokenType": firstNonEmpty(token.TokenType, "Bearer"), "expiresAt": token.Expiry, "email": account.Email})
}

func (g *Resouces) testSendGmail(c *gin.Context) {
	principal := mustGatewayPrincipal(c)
	var req struct{ To, Subject, Body string }
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	messageID, threadID, account, err := g.google.SendGmail(c.Request.Context(), principal.AccessKey.ID, req.To, req.Subject, req.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"email": account.Email, "messageId": messageID, "threadId": threadID, "to": req.To})
}

func (g *Resouces) testCreateDriveFolder(c *gin.Context) {
	principal := mustGatewayPrincipal(c)
	var req struct{ Name string }
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "folder name is required"})
		return
	}
	folder, account, err := g.google.CreateDriveFolder(c.Request.Context(), principal.AccessKey.ID, req.Name)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"email": account.Email, "folder": folder})
}

func (g *Resouces) importTelegramBot(c *gin.Context) {
	var req struct {
		BotToken         string `json:"botToken"`
		Username         string `json:"username"`
		CreatedByAccount string `json:"createdByAccount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	bot, err := g.telegram.Import(req.BotToken, req.Username, req.CreatedByAccount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	bot.BotToken = resourcetelegram.MaskToken(bot.BotToken)
	c.JSON(http.StatusCreated, gin.H{"telegramBot": bot})
}

func (g *Resouces) listTelegramBots(c *gin.Context) {
	rows, err := g.telegram.List()
	if err != nil {
		g.internalError(c, err)
		return
	}
	for i := range rows {
		rows[i].BotToken = resourcetelegram.MaskToken(rows[i].BotToken)
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (g *Resouces) getTelegramBot(c *gin.Context) {
	principal := mustGatewayPrincipal(c)
	bot, err := g.telegram.Assign(principal.AccessKey.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no available telegram bot"})
		return
	}
	if err != nil {
		g.internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"telegramBot": bot})
}

func (g *Resouces) currentBrowser(c *gin.Context) { g.startOrResetBrowser(c, false) }
func (g *Resouces) resetBrowser(c *gin.Context)   { g.startOrResetBrowser(c, true) }

func (g *Resouces) closeBrowser(c *gin.Context) {
	principal := mustGatewayPrincipal(c)
	accessKeyID := principal.AccessKey.ID
	unlock := g.lockBrowserAccessKey(accessKeyID)
	defer unlock()

	var row schema.Browser
	if err := g.wdb.Db.Where("access_key_id = ?", accessKeyID).First(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusOK, gin.H{"closed": false, "message": "browser has not been created"})
		return
	} else if err != nil {
		g.internalError(c, err)
		return
	}

	if row.ProviderBrowserID != "" {
		if err := g.browserProvider.Stop(c.Request.Context(), row.ProviderBrowserID); err != nil && !resourcebrowser.IsBrowserProviderNotFound(err) {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
	}
	now := time.Now()
	row.ProviderBrowserID = ""
	row.CDPURL = ""
	row.LiveURL = ""
	row.Status = "stopped"
	row.ProviderStartedAt = nil
	row.ProviderTimeoutAt = nil
	row.ProviderCheckedAt = &now
	if err := g.wdb.Db.Save(&row).Error; err != nil {
		g.internalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"browser": row, "closed": true})
}

func (g *Resouces) startOrResetBrowser(c *gin.Context, reset bool) {
	principal := mustGatewayPrincipal(c)
	accessKeyID := principal.AccessKey.ID
	unlock := g.lockBrowserAccessKey(accessKeyID)
	defer unlock()
	var row schema.Browser
	err := g.wdb.Db.Where("access_key_id = ?", accessKeyID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		id, idErr := newID("brw_")
		if idErr != nil {
			g.internalError(c, idErr)
			return
		}
		row = schema.Browser{ID: id, AccessKeyID: accessKeyID, ProfileName: "access-gateway-" + accessKeyID, ProxyCountryCode: g.config.BrowserProxyCountryCode, TimeoutMinutes: g.config.BrowserTimeoutMinutes, Status: schema.StatusActive}
		if err := g.wdb.Db.Create(&row).Error; err != nil {
			g.internalError(c, err)
			return
		}
	} else if err != nil {
		g.internalError(c, err)
		return
	}
	now := time.Now()
	notNearExpiry := row.ProviderTimeoutAt == nil || time.Until(*row.ProviderTimeoutAt) > 10*time.Minute
	if !reset && row.ProviderBrowserID != "" && row.CDPURL != "" && notNearExpiry {
		if row.ProviderCheckedAt != nil && now.Sub(*row.ProviderCheckedAt) < g.config.BrowserStatusCheckInterval {
			row.LastUsedAt = &now
			_ = g.wdb.Db.Model(&row).Updates(map[string]any{"last_used_at": now}).Error
			c.JSON(200, gin.H{"browser": row, "cached": true})
			return
		}
		remote, statusErr := g.browserProvider.Get(c.Request.Context(), row.ProviderBrowserID)
		if statusErr == nil && remote.Status == "active" && remote.CDPURL != "" {
			row.CDPURL = remote.CDPURL
			row.LiveURL = remote.LiveURL
			row.Status = remote.Status
			row.ProviderStartedAt = remote.StartedAt
			row.ProviderTimeoutAt = remote.TimeoutAt
			row.ProviderCheckedAt = &now
			row.LastUsedAt = &now
			if err := g.wdb.Db.Save(&row).Error; err != nil {
				g.internalError(c, err)
				return
			}
			c.JSON(200, gin.H{"browser": row, "cached": false, "providerValidated": true})
			return
		}
		if statusErr != nil && !resourcebrowser.IsBrowserProviderNotFound(statusErr) {
			c.JSON(http.StatusBadGateway, gin.H{"error": statusErr.Error()})
			return
		}
	}
	if row.ProviderBrowserID != "" {
		if err := g.browserProvider.Stop(c.Request.Context(), row.ProviderBrowserID); err != nil && !resourcebrowser.IsBrowserProviderNotFound(err) {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
	}
	row.ProviderBrowserID = ""
	row.CDPURL = ""
	row.LiveURL = ""
	row.Status = "stopped"
	row.ProviderStartedAt = nil
	row.ProviderTimeoutAt = nil
	row.ProviderCheckedAt = &now
	if err := g.wdb.Db.Save(&row).Error; err != nil {
		g.internalError(c, err)
		return
	}
	sess, err := g.browserProvider.Start(c.Request.Context(), resourcebrowser.BrowserConfig{ProxyCountryCode: row.ProxyCountryCode, TimeoutMinutes: row.TimeoutMinutes, ProfileName: row.ProfileName, ProfileID: row.ProviderProfileID})
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error()})
		return
	}
	now = time.Now()
	row.ProviderBrowserID = sess.ID
	row.ProviderProfileID = sess.ProfileID
	row.CDPURL = sess.CDPURL
	row.LiveURL = sess.LiveURL
	row.Status = sess.Status
	row.ProviderStartedAt = sess.StartedAt
	row.ProviderTimeoutAt = sess.TimeoutAt
	row.ProviderCheckedAt = &now
	row.LastUsedAt = &now
	if err := g.wdb.Db.Save(&row).Error; err != nil {
		g.internalError(c, err)
		return
	}
	c.JSON(200, gin.H{"browser": row, "reset": reset})
}

func (g *Resouces) lockBrowserAccessKey(accessKeyID string) func() {
	g.browserMu.Lock()
	lock := g.browserLocks[accessKeyID]
	if lock == nil {
		lock = &sync.Mutex{}
		g.browserLocks[accessKeyID] = lock
	}
	g.browserMu.Unlock()
	lock.Lock()
	return lock.Unlock
}

func (g *Resouces) requireGatewayAPIKey(c *gin.Context) {
	raw := firstNonEmpty(bearer(c.GetHeader("Authorization")), c.GetHeader("X-Gateway-API-Key"))
	if raw == "" {
		c.AbortWithStatusJSON(401, gin.H{"error": "gateway api key is required"})
		return
	}
	var key schema.AccessKey
	if err := g.wdb.Db.Where("key_hash = ? AND status = ?", hashSecret(raw), schema.StatusActive).First(&key).Error; err != nil {
		c.AbortWithStatusJSON(401, gin.H{"error": "invalid gateway api key"})
		return
	}
	now := time.Now()
	_ = g.wdb.Db.Model(&key).Update("last_used_at", now).Error
	c.Set(gatewayPrincipalContext, gatewayPrincipal{AccessKey: key})
}

func mustGatewayPrincipal(c *gin.Context) gatewayPrincipal {
	return c.MustGet(gatewayPrincipalContext).(gatewayPrincipal)
}
func (g *Resouces) internalError(c *gin.Context, err error) {
	log.Error("request failed", "err", err)
	c.JSON(500, gin.H{"error": "internal server error"})
}
func bearer(v string) string {
	if strings.HasPrefix(v, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(v, "Bearer "))
	}
	return ""
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
