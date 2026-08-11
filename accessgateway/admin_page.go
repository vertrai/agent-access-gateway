package accessgateway

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed web/admin.html
var adminHTML []byte

//go:embed web/test.html
var adminTestHTML []byte

func (g *AccessGateway) adminPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", adminHTML)
}

func (g *AccessGateway) adminTestPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", adminTestHTML)
}
