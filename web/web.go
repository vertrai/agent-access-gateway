// Package web owns the administration frontend shared by the resouces and
// manager services. Backend packages mount these routes without owning copies
// of the frontend assets.
package web

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed admin.html
var adminHTML []byte

//go:embed test.html
var testHTML []byte

//go:embed users.html
var usersHTML []byte

//go:embed google.html
var googleHTML []byte

//go:embed browser.html
var browserHTML []byte

//go:embed telegram.html
var telegramHTML []byte

//go:embed hymatrix.html
var hymatrixHTML []byte

//go:embed common.css
var commonCSS []byte

//go:embed common.js
var commonJS []byte

//go:embed admin-enhancements.css
var adminEnhancementsCSS []byte

// RegisterRoutes mounts the shared administration frontend on a backend.
func RegisterRoutes(routes gin.IRoutes) {
	routes.GET("/admin", adminPage)
	routes.GET("/admin/users", func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", usersHTML) })
	routes.GET("/admin/google", func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", googleHTML) })
	routes.GET("/admin/browser", func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", browserHTML) })
	routes.GET("/admin/telegram", func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", telegramHTML) })
	routes.GET("/admin/hymatrix", func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", hymatrixHTML) })
	routes.GET("/admin/test", testPage)
	routes.GET("/admin/assets/common.css", func(c *gin.Context) { c.Data(http.StatusOK, "text/css; charset=utf-8", commonCSS) })
	routes.GET("/admin/assets/admin-enhancements.css", func(c *gin.Context) { c.Data(http.StatusOK, "text/css; charset=utf-8", adminEnhancementsCSS) })
	routes.GET("/admin/assets/common.js", func(c *gin.Context) { c.Data(http.StatusOK, "application/javascript; charset=utf-8", commonJS) })
}

func adminPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", adminHTML)
}

func testPage(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", testHTML)
}
