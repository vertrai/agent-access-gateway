package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	for _, path := range []string{"/admin", "/admin/users", "/admin/google", "/admin/browser", "/admin/telegram", "/admin/weixin", "/admin/hymatrix", "/admin/test"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d", path, recorder.Code)
		}
		if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Fatalf("GET %s content type = %q", path, got)
		}
	}
}

func TestAdminPagesShareRuntimeHubBrandAndNavigation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	for _, path := range []string{"/admin", "/admin/users", "/admin/google", "/admin/browser", "/admin/telegram", "/admin/weixin", "/admin/hymatrix", "/admin/test"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		body := recorder.Body.String()
		for _, expected := range []string{"AGENT RUNTIME CONTROL", "Agent Runtime Hub", `href="/admin/browser"`, `href="/admin/weixin"`, `href="/admin/test"`} {
			if !strings.Contains(body, expected) {
				t.Errorf("GET %s is missing shared navigation content %q", path, expected)
			}
		}
	}
}

func TestWeixinPageIsStandaloneLocalHermesTest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/weixin", nil))
	for _, expected := range []string{"Weixin Bot 授权", "一次性分配给 Hermes Pod", `id="userId"`, "WEIXIN_ACCOUNT_ID", "/v1/admin/weixin/onboarding", "复制 .env"} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Errorf("Weixin page is missing %q", expected)
		}
	}
}

func TestWeixinPageRevealsAndFocusesEnvironmentAfterScan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/weixin", nil))
	body := recorder.Body.String()
	for _, expected := range []string{`id="resultCard" class="card weixin-result"`, `$("resultCard").hidden=false`, `$("resultCard").scrollIntoView({behavior:"smooth",block:"start"})`} {
		if !strings.Contains(body, expected) {
			t.Errorf("Weixin success flow is missing %q", expected)
		}
	}
	if strings.Contains(body, `id="resultCard" class="card result"`) {
		t.Error("Weixin result card uses the globally hidden .result class")
	}
}

func TestAdminPagesAutoLoadWithStoredManagerKey(t *testing.T) {
	for path, expected := range map[string]string{
		"/admin":          "if(currentManagerAdminKey())load()",
		"/admin/users":    "if (currentManagerAdminKey()) load();",
		"/admin/google":   "if(currentManagerAdminKey())load()",
		"/admin/browser":  "if(currentManagerAdminKey())loadBrowserResources()",
		"/admin/telegram": "loadTelegramResources()",
		"/admin/hymatrix": "if (currentManagerAdminKey()) load()",
	} {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		RegisterRoutes(router)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Errorf("GET %s does not auto-load data with a stored Manager key", path)
		}
	}
}

func TestBrowserResourcePageExplainsOnDemandInventory(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/browser", nil))
	body := recorder.Body.String()
	for _, expected := range []string{
		"Browser 资源池",
		"按需创建",
		"已创建 Profile",
		"活跃会话",
		"/v1/admin/browser/sessions",
		"打开 Live View",
		"closeBrowser",
		"CDP URL 属于自动化连接凭证，继续保持隐藏",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("browser resource page is missing %q", expected)
		}
	}
}

func TestUsersPageExplainsExistingAndNewUserIssuance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/users", nil))
	body := recorder.Body.String()
	for _, expected := range []string{
		"用户与访问密钥",
		"填写已有用户 ID 可追加一把 Key",
		"填写新的用户 ID 会同时创建用户",
		"完整 API Key",
		"资源权限可在签发后调整",
		`id="allowGoogle"`,
		`id="allowBrowser"`,
		`id="allowTelegram"`,
		`class="grid key-form"`,
		`class="field-help"`,
		`data-edit-scopes`,
		`class="key-result issued-key"`,
		"仅显示一次",
		`id="newKey"`,
		`id="gatewayUrl"`,
		`id="copyGatewayUrl"`,
		"window.location.origin",
		"复制 Gateway URL",
		"离开或刷新页面后，将无法再次查看完整 Key",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("users page is missing issuance guidance %q", expected)
		}
	}
}

func TestResourceAPITestUsesAdminShell(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/test", nil))
	body := recorder.Body.String()
	for _, expected := range []string{`class="shell"`, `class="side"`, `class="active" href="/admin/test"`, `/admin/assets/common.css`} {
		if !strings.Contains(body, expected) {
			t.Errorf("Resources API test page is missing %q", expected)
		}
	}
	if strings.Contains(body, "返回 Manager 控制台") {
		t.Error("Resources API test page still renders the standalone-page return button")
	}
}

func TestHymatrixPageIncludesLiveTransactionPreview(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/hymatrix", nil))
	body := recorder.Body.String()
	for _, expected := range []string{"待发送交易", "Envelope", "Protocol tags", "Container environment", "Hub-Spawn-Timestamp", "Container-Env-HERMES_AGENT_LLM_API_KEY", "显示敏感值", "复制预览", "telegramBotLink", "telegramAcquireHint", "当前 API Key 未开通 Telegram 资源", "/v1/admin/telegram/bot-link", "01 · Pod 基础配置", "02 · Node 配置", "http://52.220.233.136:8081", "G0hsaVf5gKq25JoIsEGzPfGIcKNKZofgX3i2gDivQjU", "randomPodName", "advanced-config"} {
		if !strings.Contains(body, expected) {
			t.Errorf("Hymatrix page is missing %q", expected)
		}
	}
}

func TestHymatrixPageSupportsIndependentTelegramAndWeixinChannels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/hymatrix", nil))
	body := recorder.Body.String()
	for _, expected := range []string{
		`id="enableTelegram"`, `id="enableWeixin"`, `id="weixinBotId"`,
		`/v1/admin/weixin/bots?userId=`, `enableTelegram: $("enableTelegram").checked`,
		`weixinBotId: $("enableWeixin").checked`,
		"Container-Env-HERMES_AGENT_WEIXIN_TOKEN",
		`id="authorizeWeixin"`, `id="weixinAuthDialog"`, `id="weixinAuthQR"`,
		`class="weixin-auth-body"`, `class="weixin-qr-stage"`,
		`/v1/admin/weixin/onboarding`, `function pollWeixinAuthorization`,
		`$("weixinBotId").value = state.botId`,
		`error.status === 404 || error.status === 410`,
		`$("weixinBotId").value !== state.botId`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("Hymatrix channel selection is missing %q", expected)
		}
	}
}

func TestHymatrixSpawnRequiresCompleteForm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/hymatrix", nil))
	body := recorder.Body.String()
	for _, expected := range []string{
		`id="spawn" type="submit" disabled`,
		"function isSpawnFormComplete()",
		`"botToken"`,
		`"scheduler"`,
		`"llmProvider"`,
		`$("form").checkValidity()`,
		`$("spawn").disabled = spawnSubmitting || !ready`,
		`spawnRequirements`,
		`data-missing-field`,
		`createAccessKeyLink`,
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("Hymatrix complete-form validation is missing %q", expected)
		}
	}
}

func TestHymatrixPodHistorySupportsFilteringAndDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/hymatrix", nil))
	body := recorder.Body.String()
	for _, expected := range []string{
		`id="podStatusFilter"`,
		`id="podPagination"`,
		`id="podDetail"`,
		"function filteredPods()",
		"function openPodDetail(id)",
		"失败原因",
		"data-pod-detail",
	} {
		if !strings.Contains(body, expected) {
			t.Errorf("Hymatrix Pod history is missing %q", expected)
		}
	}
}

func TestAdminEnhancementsAssetIncludesMobileNavigationStyles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/assets/admin-enhancements.css", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("enhancement stylesheet status = %d", recorder.Code)
	}
	for _, expected := range []string{".nav-toggle", ".side.menu-open .nav", ".pill.failed"} {
		if !strings.Contains(recorder.Body.String(), expected) {
			t.Errorf("enhancement stylesheet is missing %q", expected)
		}
	}
}
