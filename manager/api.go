package manager

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	gatewayweb.RegisterRoutes(r)
	admin := r.Group("/v1/admin", m.requireAdmin)
	admin.POST("/users", m.createUserAccessKey)
	admin.GET("/users", m.listUsers)
	admin.GET("/users/options", m.listUserOptions)
	admin.GET("/users/:userId/access-keys/available", m.listAvailableAccessKeys)
	admin.POST("/access-keys/:id/telegram-bot", m.acquireTelegramBot)
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/google/accounts"}, {http.MethodPost, "/google/accounts/batch"}, {http.MethodGet, "/google/accounts"},
		{http.MethodPost, "/telegram/bots"}, {http.MethodGet, "/telegram/bots"}, {http.MethodPost, "/telegram/bots/create"},
		{http.MethodPost, "/telegram/auth/init"}, {http.MethodPost, "/telegram/auth/verify"}, {http.MethodPost, "/telegram/auth/2fa"}, {http.MethodGet, "/telegram/auth/status"}, {http.MethodGet, "/telegram/auth/accounts"},
	} {
		admin.Handle(route.method, route.path, m.proxyResource("/v1/internal"+route.path))
	}
	admin.POST("/hymatrix/pods", m.spawnPod)
	admin.GET("/hymatrix/pods", m.listPods)
	admin.POST("/hymatrix/pods/:id/start", m.startPod)
	admin.POST("/hymatrix/pods/:id/stop", m.stopPod)
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
func (m *Manager) requireAdmin(c *gin.Context) {
	got := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	if got == "" {
		got = c.GetHeader("X-Admin-API-Key")
	}
	want := m.config.AdminAPIKey
	if want == "" || len(got) != len(want) || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "valid manager admin api key is required"})
	}
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
		UserID string `json:"userId"`
		Name   string `json:"name"`
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
	created, _, err := m.resources.createAccessKey(c.Request.Context(), req.UserID)
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
	token, err := m.resources.telegramBot(c.Request.Context(), key.Secret)
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"botToken": token})
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
		NodeURL, PrivateKey, Module, Scheduler           string
		LLM                                              struct {
			APIKey   string `json:"apiKey"`
			BaseURL  string `json:"baseUrl"`
			Model    string `json:"model"`
			Provider string `json:"provider"`
		} `json:"llm"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.UserID) == "" || strings.TrimSpace(req.RuntimeType) == "" || strings.TrimSpace(req.AccessKeyID) == "" || strings.TrimSpace(req.NodeURL) == "" || strings.TrimSpace(req.PrivateKey) == "" || strings.TrimSpace(req.Module) == "" || strings.TrimSpace(req.Scheduler) == "" {
		c.JSON(400, gin.H{"error": "userId, runtimeType, accessKeyId, nodeUrl, privateKey, module and scheduler are required"})
		return
	}
	var accessKey schema.AccessKey
	if err := m.wdb.Db.Where("id = ? AND user_id = ? AND status = ?", req.AccessKeyID, req.UserID, "available").First(&accessKey).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusConflict, gin.H{"error": "access key is unavailable, belongs to another user, or is already assigned"})
		return
	} else if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	config := HymatrixConfig{NodeURL: req.NodeURL, PrivateKey: req.PrivateKey, Module: req.Module, Scheduler: req.Scheduler, LLMAPIKey: req.LLM.APIKey, LLMBaseURL: req.LLM.BaseURL, LLMModel: req.LLM.Model, LLMProvider: req.LLM.Provider}
	client, err := NewHymatrixClient(config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pid, err := client.Spawn(c.Request.Context(), req.RuntimeType)
	pod := schema.HymatrixPod{ID: "pod_" + strings.ReplaceAll(uuid.NewString(), "-", ""), UserID: req.UserID, Name: req.Name, RuntimeType: req.RuntimeType, PID: pid, Status: schema.PodStatusSpawned, NodeURL: req.NodeURL, PrivateKey: req.PrivateKey, Module: req.Module, Scheduler: req.Scheduler, LLMAPIKey: req.LLM.APIKey, LLMBaseURL: req.LLM.BaseURL, LLMModel: req.LLM.Model, LLMProvider: req.LLM.Provider, GatewayAPIKey: accessKey.Secret, AccessKeyID: accessKey.ID, BotToken: req.BotToken}
	if pod.Name == "" {
		pod.Name = req.RuntimeType
	}
	if err != nil {
		pod.Status = schema.PodStatusFailed
		pod.Error = err.Error()
	}
	if saveErr := m.wdb.Db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&pod).Error; err != nil {
			return err
		}
		result := tx.Model(&schema.AccessKey{}).Where("id = ? AND status = ?", accessKey.ID, "available").Updates(map[string]any{"status": "assigned", "assigned_pod_id": pod.ID})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("access key was assigned concurrently")
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
func (m *Manager) pod(c *gin.Context) (schema.HymatrixPod, error) {
	var pod schema.HymatrixPod
	err := m.wdb.Db.First(&pod, "id = ?", c.Param("id")).Error
	return pod, err
}
func (m *Manager) startPod(c *gin.Context) {
	pod, err := m.pod(c)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, gin.H{"error": "pod not found"})
		return
	} else if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	bot := pod.BotToken
	if bot == "" && pod.RuntimeType == "telegramcustomer" {
		bot, err = m.resources.telegramBot(c.Request.Context(), pod.GatewayAPIKey)
	}
	if err == nil {
		client, clientErr := NewHymatrixClient(podHymatrixConfig(pod))
		if clientErr != nil {
			err = clientErr
		} else {
			err = client.Start(c.Request.Context(), PodStartInput{PID: pod.PID, GatewayURL: m.config.Resources.BaseURL, GatewayAPIKey: pod.GatewayAPIKey, BotToken: bot})
		}
	}
	if err != nil {
		pod.Status = schema.PodStatusFailed
		pod.Error = err.Error()
	} else {
		pod.Status = schema.PodStatusRunning
		pod.Error = ""
	}
	_ = m.wdb.Db.Save(&pod).Error
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error(), "pod": pod})
		return
	}
	c.JSON(200, gin.H{"pod": pod})
}
func (m *Manager) stopPod(c *gin.Context) {
	pod, err := m.pod(c)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, gin.H{"error": "pod not found"})
		return
	} else if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	client, clientErr := NewHymatrixClient(podHymatrixConfig(pod))
	if clientErr != nil {
		err = clientErr
	} else {
		err = client.Stop(c.Request.Context(), pod.PID)
	}
	if err != nil {
		pod.Status = schema.PodStatusFailed
		pod.Error = err.Error()
	} else {
		pod.Status = schema.PodStatusStopped
		pod.Error = ""
	}
	_ = m.wdb.Db.Save(&pod).Error
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error(), "pod": pod})
		return
	}
	c.JSON(200, gin.H{"pod": pod})
}

func podHymatrixConfig(pod schema.HymatrixPod) HymatrixConfig {
	return HymatrixConfig{NodeURL: pod.NodeURL, PrivateKey: pod.PrivateKey, Module: pod.Module, Scheduler: pod.Scheduler, LLMAPIKey: pod.LLMAPIKey, LLMBaseURL: pod.LLMBaseURL, LLMModel: pod.LLMModel, LLMProvider: pod.LLMProvider}
}
