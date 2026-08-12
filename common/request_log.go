package common

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inconshreveable/log15"
)

// RequestLogger writes one structured record after every HTTP request. It
// deliberately logs URL.Path instead of RequestURI so query-string secrets do
// not accidentally reach application logs.
func RequestLogger(log log15.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		log.Info("http request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency", time.Since(started),
			"clientIP", c.ClientIP(),
			"errors", c.Errors.String(),
		)
	}
}
