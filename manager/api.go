package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/vertrai/hub/common"
	"github.com/vertrai/hub/manager/schema"
	gatewayweb "github.com/vertrai/hub/web"
	"gorm.io/gorm"
)

func (m *Manager) router() *gin.Engine {
	r := gin.New()
	r.Use(common.RequestLogger(log), gin.Recovery(), common.CORSMiddleware())
	r.GET("/info", m.info)
	gatewayweb.RegisterRoutes(r, m.requireAdminPage)
	r.GET("/v1/admin/auth/info", m.adminAuthInfo)
	r.POST("/v1/admin/auth/google", m.adminGoogleLogin)
	r.GET("/v1/admin/session", m.adminMe)
	r.POST("/v1/admin/logout", m.adminLogout)
	admin := r.Group("/v1/admin", m.requireAdmin)
	admin.POST("/users", m.createUserAccessKey)
	admin.GET("/users", m.listUsers)
	admin.PATCH("/access-keys/:id/scopes", m.updateAccessKeyScopes)
	admin.GET("/users/options", m.listUserOptions)
	admin.GET("/users/:userId/access-keys/available", m.listAvailableAccessKeys)
	admin.POST("/access-keys/:id/telegram-bot", m.acquireTelegramBot)
	admin.POST("/telegram/bot-link", m.resolveTelegramBotLink)
	admin.POST("/browser/sessions/:id/close", func(c *gin.Context) {
		m.proxyResource("/v1/internal/browser/sessions/" + url.PathEscape(c.Param("id")) + "/close")(c)
	})
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/google/accounts"}, {http.MethodPost, "/google/accounts/batch"}, {http.MethodGet, "/google/accounts"},
		{http.MethodGet, "/browser/sessions"},
		{http.MethodPost, "/telegram/bots"}, {http.MethodGet, "/telegram/bots"}, {http.MethodPost, "/telegram/bots/create"},
		{http.MethodPost, "/telegram/auth/init"}, {http.MethodPost, "/telegram/auth/verify"}, {http.MethodPost, "/telegram/auth/2fa"}, {http.MethodGet, "/telegram/auth/status"}, {http.MethodGet, "/telegram/auth/accounts"},
	} {
		admin.Handle(route.method, route.path, m.proxyResource("/v1/internal"+route.path))
	}
	admin.POST("/hymatrix/pods", m.spawnPod)
	admin.GET("/hymatrix/pods", m.listPods)
	admin.GET("/hymatrix/node-info", m.hymatrixNodeInfo)
	admin.POST("/weixin/onboarding", m.startWeixinOnboarding)
	admin.GET("/weixin/onboarding/:attempt", m.pollWeixinOnboarding)
	admin.GET("/weixin/onboarding/:attempt/credentials", m.getWeixinOnboardingCredentials)
	admin.DELETE("/weixin/onboarding/:attempt", m.cancelWeixinOnboarding)
	admin.GET("/weixin/bots", m.listWeixinBots)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/google-user"}, {http.MethodGet, "/google-user/access-token"},
		{http.MethodPost, "/google-user/test/gmail/send"}, {http.MethodPost, "/google-user/test/drive/folders"},
		{http.MethodGet, "/browser"}, {http.MethodPost, "/browser/reset"}, {http.MethodPost, "/browser/close"},
		{http.MethodGet, "/telegram-bot"},
	} {
		r.Handle(route.method, "/v1"+route.path, m.proxyGatewayResource("/v1"+route.path))
	}
	return r
}

func (m *Manager) runAPI(endpoint string) {
	m.apiServer = &http.Server{Addr: endpoint, Handler: m.router()}
	if err := m.apiServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("api server stopped", "err", err)
	}
}
func (m *Manager) info(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"service": "manager", "status": "ok", "resourcesConfigured": m.resources.configured()})
}
func (m *Manager) proxyResource(path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body any
		if c.Request.Body != nil && c.Request.ContentLength != 0 {
			var raw json.RawMessage
			if err := c.ShouldBindJSON(&raw); err != nil {
				c.JSON(400, gin.H{"error": "invalid request body"})
				return
			}
			body = raw
		}
		raw, status, err := m.resources.do(c.Request.Context(), c.Request.Method, path, body, "")
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		ctype := "application/json"
		c.Data(status, ctype, raw)
	}
}

func (m *Manager) proxyGatewayResource(path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
		if key == "" {
			key = c.GetHeader("X-Gateway-API-Key")
		}
		if key == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "gateway api key is required"})
			return
		}
		var body any
		if c.Request.Body != nil && c.Request.ContentLength != 0 {
			var raw json.RawMessage
			if err := c.ShouldBindJSON(&raw); err != nil {
				c.JSON(400, gin.H{"error": "invalid request body"})
				return
			}
			body = raw
		}
		raw, status, err := m.resources.do(c.Request.Context(), c.Request.Method, path, body, key)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		c.Data(status, "application/json", raw)
	}
}

func (m *Manager) createUserAccessKey(c *gin.Context) {
	var req struct {
		UserID        string `json:"userId"`
		Name          string `json:"name"`
		AllowGoogle   *bool  `json:"allowGoogle"`
		AllowBrowser  *bool  `json:"allowBrowser"`
		AllowTelegram *bool  `json:"allowTelegram"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.UserID) == "" {
		c.JSON(400, gin.H{"error": "userId is required"})
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	if req.Name == "" {
		req.Name = req.UserID
	}
	if m.wdb == nil {
		c.JSON(503, gin.H{"error": "manager database is unavailable"})
		return
	}
	user := schema.User{ID: req.UserID, Name: req.Name, Status: "active"}
	if err := m.wdb.Db.Where("id = ?", user.ID).Assign(schema.User{Name: user.Name, Status: user.Status}).FirstOrCreate(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	created, _, err := m.resources.createAccessKey(c.Request.Context(), req.UserID, ResourceScopes{
		AllowGoogle: boolDefaultTrue(req.AllowGoogle), AllowBrowser: boolDefaultTrue(req.AllowBrowser), AllowTelegram: boolDefaultTrue(req.AllowTelegram),
	})
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error()})
		return
	}
	local := schema.AccessKey{ID: "mak_" + strings.ReplaceAll(uuid.NewString(), "-", ""), UserID: req.UserID, ResourceKeyID: created.AccessKey.ID, KeyPrefix: created.AccessKey.KeyPrefix, Secret: created.GatewayAPIKey, Status: "available"}
	if err := m.wdb.Db.Create(&local).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accessKey": created.AccessKey, "gatewayApiKey": created.GatewayAPIKey})
}

func boolDefaultTrue(value *bool) bool {
	return value == nil || *value
}

func (m *Manager) updateAccessKeyScopes(c *gin.Context) {
	var scopes ResourceScopes
	if err := c.ShouldBindJSON(&scopes); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid resource scopes"})
		return
	}
	key, status, err := m.resources.updateAccessKeyScopes(c.Request.Context(), c.Param("id"), scopes)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(status, gin.H{"accessKey": key})
}

func (m *Manager) listUserOptions(c *gin.Context) {
	var users []schema.User
	if m.wdb == nil {
		c.JSON(503, gin.H{"error": "manager database is unavailable"})
		return
	}
	if err := m.wdb.Db.Where("status = ?", "active").Order("name asc").Find(&users).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": users})
}
func (m *Manager) listAvailableAccessKeys(c *gin.Context) {
	var keys []schema.AccessKey
	if m.wdb == nil {
		c.JSON(503, gin.H{"error": "manager database is unavailable"})
		return
	}
	if err := m.wdb.Db.Where("user_id = ? AND status = ?", c.Param("userId"), "available").Order("created_at desc").Find(&keys).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": keys})
}
func (m *Manager) acquireTelegramBot(c *gin.Context) {
	var key schema.AccessKey
	if m.wdb == nil {
		c.JSON(503, gin.H{"error": "manager database is unavailable"})
		return
	}
	if err := m.wdb.Db.Where("id = ? AND status = ?", c.Param("id"), "available").First(&key).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(409, gin.H{"error": "access key is unavailable or already assigned"})
		return
	} else if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	bot, err := m.resources.telegramBotDetails(c.Request.Context(), key.Secret)
	if err != nil {
		var resourcesError *ResourcesHTTPError
		if errors.As(err, &resourcesError) && resourcesError.StatusCode == http.StatusForbidden {
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "当前 API Key 未开通 Telegram 资源，请手动填写 Bot Token 或调整 API Key 权限",
				"resource": "telegram",
			})
			return
		}
		c.JSON(502, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"botToken": bot.BotToken, "username": bot.Username, "botLink": telegramBotLink(bot.Username)})
}

func (m *Manager) listUsers(c *gin.Context) {
	if m.wdb == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "manager database is unavailable"})
		return
	}
	var users []schema.User
	if err := m.wdb.Db.Order("created_at desc").Find(&users).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	keys, err := m.resources.listAccessKeys(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	type userSummary struct {
		schema.User
		AccessKeys []ResourceAccessKey `json:"accessKeys"`
	}
	items := make([]userSummary, 0, len(users))
	byOwner := make(map[string][]ResourceAccessKey)
	for _, key := range keys {
		byOwner[key.OwnerUserID] = append(byOwner[key.OwnerUserID], key)
	}
	for _, user := range users {
		assigned := byOwner[user.ID]
		if assigned == nil {
			assigned = []ResourceAccessKey{}
		}
		items = append(items, userSummary{User: user, AccessKeys: assigned})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (m *Manager) spawnPod(c *gin.Context) {
	if m.wdb == nil {
		c.JSON(503, gin.H{"error": "manager database is unavailable"})
		return
	}
	var req struct {
		UserID, Name, RuntimeType, AccessKeyID, BotToken string
		EnableTelegram                                   bool   `json:"enableTelegram"`
		WeixinBotID                                      string `json:"weixinBotId"`
		GatewayURL                                       string `json:"gatewayUrl"`
		HermesGatewayToken                               string `json:"hermesGatewayToken"`
		NodeURL, PrivateKey, Module                      string
		LLM                                              struct {
			APIKey   string `json:"apiKey"`
			BaseURL  string `json:"baseUrl"`
			Model    string `json:"model"`
			Provider string `json:"provider"`
		} `json:"llm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.UserID) == "" || strings.TrimSpace(req.RuntimeType) == "" || strings.TrimSpace(req.AccessKeyID) == "" || strings.TrimSpace(req.NodeURL) == "" || strings.TrimSpace(req.PrivateKey) == "" || strings.TrimSpace(req.Module) == "" {
		c.JSON(400, gin.H{"error": "userId, runtimeType, accessKeyId, nodeUrl, privateKey and module are required"})
		return
	}
	req.GatewayURL = strings.TrimRight(strings.TrimSpace(req.GatewayURL), "/")
	req.HermesGatewayToken = strings.TrimSpace(req.HermesGatewayToken)
	req.BotToken = strings.TrimSpace(req.BotToken)
	telegramEnabled := req.EnableTelegram || req.BotToken != "" || req.RuntimeType == "telegramcustomer"
	if !telegramEnabled && strings.TrimSpace(req.WeixinBotID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one messaging channel (Telegram or Weixin) is required"})
		return
	}
	if req.HermesGatewayToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hermesGatewayToken is required"})
		return
	}
	gatewayURL, gatewayURLErr := url.Parse(req.GatewayURL)
	if gatewayURLErr != nil || gatewayURL.Host == "" || (gatewayURL.Scheme != "http" && gatewayURL.Scheme != "https") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "gatewayUrl must be an absolute HTTP or HTTPS URL"})
		return
	}
	req.LLM.Provider = strings.TrimSpace(req.LLM.Provider)
	if req.LLM.Provider == "" {
		req.LLM.Provider = "custom"
	}
	if strings.TrimSpace(req.LLM.Model) == "" || strings.TrimSpace(req.LLM.APIKey) == "" || (req.LLM.Provider == "custom" && strings.TrimSpace(req.LLM.BaseURL) == "") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "llm.model and llm.apiKey are required; llm.baseUrl is also required for custom provider"})
		return
	}
	nodeInfo, err := fetchHymatrixNodeInfo(c.Request.Context(), req.NodeURL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	scheduler := strings.TrimSpace(nodeInfo.Node.AccountID)
	var accessKey schema.AccessKey
	if err := m.wdb.Db.Where("id = ? AND user_id = ? AND status = ?", req.AccessKeyID, req.UserID, "available").First(&accessKey).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusConflict, gin.H{"error": "access key is unavailable, belongs to another user, or is already assigned"})
		return
	} else if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	var weixinBot schema.WeixinBot
	req.WeixinBotID = strings.TrimSpace(req.WeixinBotID)
	if req.WeixinBotID != "" {
		if err := m.wdb.Db.Where("id = ? AND user_id = ? AND status = ?", req.WeixinBotID, req.UserID, schema.WeixinBotStatusAvailable).First(&weixinBot).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusConflict, gin.H{"error": "Weixin bot is unavailable, belongs to another user, or is already assigned"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	config := HymatrixConfig{NodeURL: req.NodeURL, PrivateKey: req.PrivateKey, Module: req.Module, Scheduler: scheduler, LLMAPIKey: req.LLM.APIKey, LLMBaseURL: req.LLM.BaseURL, LLMModel: req.LLM.Model, LLMProvider: req.LLM.Provider}
	client, err := NewHymatrixClient(config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	podID := "pod_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	pod := schema.HymatrixPod{ID: podID, UserID: req.UserID, Name: req.Name, RuntimeType: req.RuntimeType, PID: "pending_" + podID, Status: schema.PodStatusSpawning, NodeURL: req.NodeURL, PrivateKey: req.PrivateKey, Module: req.Module, Scheduler: scheduler, LLMAPIKey: req.LLM.APIKey, LLMBaseURL: req.LLM.BaseURL, LLMModel: req.LLM.Model, LLMProvider: req.LLM.Provider, GatewayAPIKey: accessKey.Secret, AccessKeyID: accessKey.ID, BotToken: strings.TrimSpace(req.BotToken), WeixinBotID: req.WeixinBotID}
	if pod.Name == "" {
		pod.Name = req.RuntimeType
	}
	if reserveErr := m.wdb.Db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&schema.AccessKey{}).Where("id = ? AND status = ?", accessKey.ID, "available").Updates(map[string]any{"status": "assigned", "assigned_pod_id": pod.ID})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("access key was assigned concurrently")
		}
		if req.WeixinBotID != "" {
			result = tx.Model(&schema.WeixinBot{}).Where("id = ? AND user_id = ? AND status = ?", req.WeixinBotID, req.UserID, schema.WeixinBotStatusAvailable).Updates(map[string]any{"status": schema.WeixinBotStatusAssigned, "assigned_pod_id": pod.ID})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("Weixin bot was assigned concurrently")
			}
		}
		return tx.Create(&pod).Error
	}); reserveErr != nil {
		c.JSON(http.StatusConflict, gin.H{"error": reserveErr.Error()})
		return
	}
	botToken := pod.BotToken
	if botToken == "" && telegramEnabled {
		botToken, err = m.resources.telegramBot(c.Request.Context(), accessKey.Secret)
	}
	var pid string
	if err == nil {
		pid, err = client.Spawn(c.Request.Context(), PodSpawnInput{
			RuntimeType: req.RuntimeType, GatewayURL: req.GatewayURL, GatewayAPIKey: accessKey.Secret,
			BotToken: botToken, HermesGatewayToken: req.HermesGatewayToken,
			WeixinAccountID: weixinBot.AccountID, WeixinToken: weixinBot.Token,
			WeixinBaseURL: weixinBot.BaseURL, WeixinAllowedUsers: weixinBot.AllowedUserID,
		})
	}
	if err != nil {
		pod.Status = schema.PodStatusFailed
		pod.Error = err.Error()
	} else {
		pod.PID = pid
		pod.Status = schema.PodStatusSpawned
		pod.BotToken = botToken
	}
	if saveErr := m.wdb.Db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&pod).Error; err != nil {
			return err
		}
		if err != nil {
			if releaseErr := tx.Model(&schema.AccessKey{}).Where("id = ? AND assigned_pod_id = ?", accessKey.ID, pod.ID).Updates(map[string]any{"status": "available", "assigned_pod_id": nil}).Error; releaseErr != nil {
				return releaseErr
			}
			if req.WeixinBotID != "" {
				return tx.Model(&schema.WeixinBot{}).Where("id = ? AND assigned_pod_id = ?", req.WeixinBotID, pod.ID).Updates(map[string]any{"status": schema.WeixinBotStatusAvailable, "assigned_pod_id": nil}).Error
			}
		}
		return nil
	}); saveErr != nil {
		c.JSON(500, gin.H{"error": saveErr.Error()})
		return
	}
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error(), "pod": pod})
		return
	}
	c.JSON(201, gin.H{"pod": pod})
}

func (m *Manager) hymatrixNodeInfo(c *gin.Context) {
	nodeURL := strings.TrimSpace(c.Query("nodeUrl"))
	if nodeURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nodeUrl is required"})
		return
	}
	info, err := fetchHymatrixNodeInfo(c.Request.Context(), nodeURL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"scheduler":   strings.TrimSpace(info.Node.AccountID),
		"nodeName":    info.Node.Name,
		"nodeVersion": info.NodeVersion,
		"protocol":    info.Protocol,
	})
}
func (m *Manager) listPods(c *gin.Context) {
	var pods []schema.HymatrixPod
	if m.wdb == nil {
		c.JSON(503, gin.H{"error": "manager database is unavailable"})
		return
	}
	if err := m.wdb.Db.Order("created_at desc").Find(&pods).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": pods})
}
