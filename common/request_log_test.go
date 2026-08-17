package common

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/inconshreveable/log15"
)

func TestRequestLoggerWritesRequestDetailsWithoutQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := log15.New()
	logger.SetHandler(log15.StreamHandler(&output, log15.LogfmtFormat()))
	router := gin.New()
	router.Use(RequestLogger(logger))
	router.GET("/info", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/info?apiKey=must-not-be-logged", nil)
	router.ServeHTTP(recorder, request)
	logged := output.String()
	for _, expected := range []string{"http request", "method=GET", "path=/info", "status=204"} {
		if !strings.Contains(logged, expected) {
			t.Fatalf("log %q does not contain %q", logged, expected)
		}
	}
	if strings.Contains(logged, "must-not-be-logged") {
		t.Fatalf("request query secret was logged: %q", logged)
	}
}
