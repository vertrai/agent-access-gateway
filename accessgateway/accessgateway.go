package accessgateway

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/zyjblockchain/agent-access-gateway/common"
)

var log = common.NewLog("access-gateway")

type Config struct {
	AdminAPIKey                    string
	BrowserAPIKey                  string
	BrowserAPIBaseURL              string
	BrowserTimeoutMinutes          int
	BrowserProxyCountryCode        string
	BrowserStatusCheckInterval     time.Duration
	GoogleCreationCredentials      string
	GoogleCreationAdminEmail       string
	GoogleCreationDomain           string
	GoogleAuthorizationCredentials string
	GoogleAuthorizationDomain      string
	GoogleAuthorizationScopes      []string
}

type AccessGateway struct {
	env             string
	config          Config
	wdb             *Wdb
	scheduler       *gocron.Scheduler
	apiServer       *http.Server
	browserProvider BrowserProvider
	googleCreator   GoogleUserCreator
	tokenIssuer     GoogleTokenIssuer
	browserMu       sync.Mutex
	browserLocks    map[string]*sync.Mutex
}

func New(env string, config Config, wdb *Wdb) *AccessGateway {
	if config.BrowserTimeoutMinutes <= 0 {
		config.BrowserTimeoutMinutes = 240
	}
	if config.BrowserProxyCountryCode == "" {
		config.BrowserProxyCountryCode = "us"
	}
	if config.BrowserAPIBaseURL == "" {
		config.BrowserAPIBaseURL = "https://api.browser-use.com/api/v3"
	}
	if config.BrowserStatusCheckInterval <= 0 {
		config.BrowserStatusCheckInterval = 30 * time.Second
	}
	g := &AccessGateway{env: env, config: config, wdb: wdb, scheduler: gocron.NewScheduler(time.Local), browserLocks: make(map[string]*sync.Mutex)}
	g.browserProvider = NewBrowserUseProvider(config.BrowserAPIKey, config.BrowserAPIBaseURL)
	g.googleCreator = NewWorkspaceAdminCreator(config.GoogleCreationCredentials, config.GoogleCreationAdminEmail)
	g.tokenIssuer = NewCachedGoogleTokenIssuer(
		NewDWDTokenIssuer(config.GoogleAuthorizationCredentials, config.GoogleAuthorizationDomain, config.GoogleAuthorizationScopes),
		5*time.Minute,
	)
	return g
}

func (g *AccessGateway) Run(endpoint string) { go g.runJobs(); go g.runAPI(endpoint) }

func (g *AccessGateway) Close() {
	g.scheduler.Stop()
	if g.apiServer != nil {
		_ = g.apiServer.Shutdown(context.Background())
	}
	if g.wdb != nil {
		_ = g.wdb.Close()
	}
}
