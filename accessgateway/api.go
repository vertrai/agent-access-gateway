package accessgateway

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zyjblockchain/agent-access-gateway/accessgateway/schema"
	"github.com/zyjblockchain/agent-access-gateway/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const gatewayPrincipalContext = "gatewayPrincipal"

type gatewayPrincipal struct {
	User      schema.GatewayUser
	AccessKey schema.AccessKey
}

func (g *AccessGateway) router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), common.CORSMiddleware())
	r.GET("/info", g.info)
	r.GET("/admin", g.adminPage)
	r.GET("/admin/test", g.adminTestPage)
	admin := r.Group("/v1/admin", g.requireAdmin)
	admin.POST("/users", g.createUserAccessKey)
	admin.GET("/users", g.listUsers)
	admin.POST("/google/accounts", g.createGoogleAccount)
	admin.POST("/google/accounts/batch", g.createGoogleAccountsBatch)
	admin.GET("/google/accounts", g.listGoogleAccounts)
	user := r.Group("/v1", g.requireGatewayAPIKey)
	user.GET("/google-user", g.getGoogleUser)
	user.GET("/google-user/access-token", g.issueGoogleToken)
	user.POST("/google-user/test/gmail/send", g.testSendGmail)
	user.POST("/google-user/test/drive/folders", g.testCreateDriveFolder)
	user.GET("/browser", g.currentBrowser)
	user.POST("/browser/reset", g.resetBrowser)
	user.POST("/browser/close", g.closeBrowser)
	return r
}

func (g *AccessGateway) listUsers(c *gin.Context) {
	type accessKeySummary struct {
		schema.AccessKey
		GoogleEmail string `json:"googleEmail,omitempty"`
		BrowserID   string `json:"browserId,omitempty"`
	}
	type userSummary struct {
		schema.GatewayUser
		AccessKeys []accessKeySummary `json:"accessKeys"`
	}
	var users []schema.GatewayUser
	if err := g.wdb.Db.Order("created_at desc").Find(&users).Error; err != nil {
		g.internalError(c, err)
		return
	}
	items := make([]userSummary, 0, len(users))
	for _, user := range users {
		item := userSummary{GatewayUser: user, AccessKeys: []accessKeySummary{}}
		var keys []schema.AccessKey
		if err := g.wdb.Db.Where("user_id = ?", user.ID).Order("created_at desc").Find(&keys).Error; err != nil {
			g.internalError(c, err)
			return
		}
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
			item.AccessKeys = append(item.AccessKeys, keyItem)
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (g *AccessGateway) runAPI(endpoint string) {
	g.apiServer = &http.Server{Addr: endpoint, Handler: g.router()}
	if err := g.apiServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("http listen", "err", err)
	}
}
func (g *AccessGateway) info(c *gin.Context) {
	c.JSON(200, gin.H{"name": "agent-access-gateway", "env": g.env})
}
func (g *AccessGateway) requireAdmin(c *gin.Context) {
	got := firstNonEmpty(c.GetHeader("X-Admin-API-Key"), bearer(c.GetHeader("Authorization")))
	if g.config.AdminAPIKey == "" || len(got) != len(g.config.AdminAPIKey) || subtle.ConstantTimeCompare([]byte(got), []byte(g.config.AdminAPIKey)) != 1 {
		c.AbortWithStatusJSON(401, gin.H{"error": "valid admin api key is required"})
	}
}

func (g *AccessGateway) createUserAccessKey(c *gin.Context) {
	var req struct {
		UserID string `json:"userId"`
		Name   string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.UserID) == "" {
		c.JSON(400, gin.H{"error": "userId is required"})
		return
	}
	userID := strings.TrimSpace(req.UserID)
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
	row := schema.GatewayUser{ID: userID, Name: firstNonEmpty(req.Name, userID), Status: schema.StatusActive}
	accessKey := schema.AccessKey{ID: keyID, UserID: userID, KeyHash: hashSecret(key), KeyPrefix: secretPrefix(key), Status: schema.StatusActive}
	err = g.wdb.Db.Transaction(func(tx *gorm.DB) error {
		var existing schema.GatewayUser
		findErr := tx.First(&existing, "id = ?", userID).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		} else if findErr != nil {
			return findErr
		} else {
			row = existing
			row.Name = firstNonEmpty(req.Name, row.Name)
			row.Status = schema.StatusActive
			if err := tx.Save(&row).Error; err != nil {
				return err
			}
		}
		return tx.Create(&accessKey).Error
	})
	if err != nil {
		g.internalError(c, err)
		return
	}
	c.JSON(200, gin.H{"user": row, "accessKey": accessKey, "gatewayApiKey": key})
}

func (g *AccessGateway) createGoogleAccount(c *gin.Context) {
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
		req.Password, err = randomPassword()
		if err != nil {
			g.internalError(c, err)
			return
		}
	}
	row, err := g.createWorkspaceAccount(c.Request.Context(), req.Email, req.Password, firstNonEmpty(req.GivenName, "Agent"), firstNonEmpty(req.FamilyName, "User"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, gin.H{"account": row})
}

func (g *AccessGateway) createGoogleAccountsBatch(c *gin.Context) {
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
	domain := firstNonEmpty(req.Domain, g.config.GoogleCreationDomain)
	if !strings.EqualFold(domain, g.config.GoogleCreationDomain) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "domain must match configured workspace domain"})
		return
	}
	created := make([]schema.GoogleAccount, 0, req.Count)
	for len(created) < req.Count {
		email, err := g.nextGoogleAccountEmail(domain)
		if err != nil {
			c.JSON(http.StatusMultiStatus, gin.H{"error": err.Error(), "created": created, "createdCount": len(created), "requestedCount": req.Count})
			return
		}
		password, err := randomPassword()
		if err != nil {
			g.internalError(c, err)
			return
		}
		account, err := g.createWorkspaceAccount(c.Request.Context(), email, password, "Agent", strings.TrimSuffix(email, "@"+domain))
		if err != nil {
			status := http.StatusBadGateway
			if len(created) > 0 {
				status = http.StatusMultiStatus
			}
			c.JSON(status, gin.H{"error": err.Error(), "created": created, "createdCount": len(created), "requestedCount": req.Count})
			return
		}
		created = append(created, account)
	}
	c.JSON(http.StatusCreated, gin.H{"created": created, "createdCount": len(created), "requestedCount": req.Count})
}
func (g *AccessGateway) listGoogleAccounts(c *gin.Context) {
	var rows []schema.GoogleAccount
	if err := g.wdb.Db.Order("created_at desc").Find(&rows).Error; err != nil {
		g.internalError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": rows})
}

func (g *AccessGateway) getGoogleUser(c *gin.Context) {
	principal := mustGatewayPrincipal(c)
	account, err := g.assignGoogleAccount(principal.AccessKey.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(503, gin.H{"error": "no available google account"})
		return
	}
	if err != nil {
		g.internalError(c, err)
		return
	}
	c.JSON(200, gin.H{"googleUser": account})
}
func (g *AccessGateway) assignGoogleAccount(accessKeyID string) (schema.GoogleAccount, error) {
	var account schema.GoogleAccount
	err := g.wdb.Db.Transaction(func(tx *gorm.DB) error {
		var accessKey schema.AccessKey
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&accessKey, "id = ?", accessKeyID).Error; err != nil {
			return err
		}
		err := tx.Where("assigned_access_key_id = ?", accessKeyID).First(&account).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("status = ?", schema.StatusAvailable).Order("created_at").First(&account).Error; err != nil {
			return err
		}
		now := time.Now()
		account.Status = schema.StatusAssigned
		account.AssignedAccessKeyID = &accessKeyID
		account.AssignedAt = &now
		return tx.Save(&account).Error
	})
	return account, err
}

func (g *AccessGateway) issueGoogleToken(c *gin.Context) {
	principal := mustGatewayPrincipal(c)
	account, err := g.assignGoogleAccount(principal.AccessKey.ID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(503, gin.H{"error": "no available google account"})
		return
	}
	if err != nil {
		g.internalError(c, err)
		return
	}
	token, err := g.tokenIssuer.Issue(c.Request.Context(), account.Email)
	if err != nil {
		c.JSON(502, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"accessToken": token.AccessToken, "tokenType": firstNonEmpty(token.TokenType, "Bearer"), "expiresAt": token.Expiry, "email": account.Email})
}

func (g *AccessGateway) currentBrowser(c *gin.Context) { g.startOrResetBrowser(c, false) }
func (g *AccessGateway) resetBrowser(c *gin.Context)   { g.startOrResetBrowser(c, true) }

func (g *AccessGateway) closeBrowser(c *gin.Context) {
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
		if err := g.browserProvider.Stop(c.Request.Context(), row.ProviderBrowserID); err != nil && !IsBrowserProviderNotFound(err) {
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

func (g *AccessGateway) startOrResetBrowser(c *gin.Context, reset bool) {
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
		if statusErr != nil && !IsBrowserProviderNotFound(statusErr) {
			c.JSON(http.StatusBadGateway, gin.H{"error": statusErr.Error()})
			return
		}
	}
	if row.ProviderBrowserID != "" {
		if err := g.browserProvider.Stop(c.Request.Context(), row.ProviderBrowserID); err != nil && !IsBrowserProviderNotFound(err) {
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
	sess, err := g.browserProvider.Start(c.Request.Context(), BrowserConfig{ProxyCountryCode: row.ProxyCountryCode, TimeoutMinutes: row.TimeoutMinutes, ProfileName: row.ProfileName, ProfileID: row.ProviderProfileID})
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

func (g *AccessGateway) lockBrowserAccessKey(accessKeyID string) func() {
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

func (g *AccessGateway) requireGatewayAPIKey(c *gin.Context) {
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
	var user schema.GatewayUser
	if err := g.wdb.Db.Where("id = ? AND status = ?", key.UserID, schema.StatusActive).First(&user).Error; err != nil {
		c.AbortWithStatusJSON(401, gin.H{"error": "gateway user is inactive"})
		return
	}
	now := time.Now()
	_ = g.wdb.Db.Model(&key).Update("last_used_at", now).Error
	c.Set(gatewayPrincipalContext, gatewayPrincipal{User: user, AccessKey: key})
}

func mustGatewayPrincipal(c *gin.Context) gatewayPrincipal {
	return c.MustGet(gatewayPrincipalContext).(gatewayPrincipal)
}
func (g *AccessGateway) internalError(c *gin.Context, err error) {
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
