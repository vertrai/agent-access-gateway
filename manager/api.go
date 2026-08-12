package manager

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/vertrai/agent-access-gateway/common"
	gatewayweb "github.com/vertrai/agent-access-gateway/web"
)

func (m *Manager) router() *gin.Engine {
	r := gin.New()
	r.Use(common.RequestLogger(log), gin.Recovery(), common.CORSMiddleware())
	r.GET("/info", m.info)
	gatewayweb.RegisterRoutes(r)
	return r
}

func (m *Manager) runAPI(endpoint string) {
	m.apiServer = &http.Server{Addr: endpoint, Handler: m.router()}
	if err := m.apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("api server stopped", "err", err)
	}
}

func (m *Manager) info(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"service": "manager", "status": "ok"})
}
