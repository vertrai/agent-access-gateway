package manager

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/vertrai/hub/common"
)

var log = common.NewLog("manager")

type Config struct {
	AdminAPIKey string
	Resources   ResourcesConfig
}

type ResourcesConfig struct {
	BaseURL, AdminAPIKey string
	Timeout              time.Duration
}

type Manager struct {
	env            string
	config         Config
	wdb            *Wdb
	resources      *ResourcesClient
	apiServer      *http.Server
	weixinMu       sync.Mutex
	weixinAttempts map[string]weixinAttempt
	weixinBaseURL  string
	weixinClient   *http.Client
}

func New(env string, config Config, wdb *Wdb) (*Manager, error) {
	if config.Resources.Timeout <= 0 {
		config.Resources.Timeout = 30 * time.Second
	}
	return &Manager{
		env: env, config: config, wdb: wdb, resources: NewResourcesClient(config.Resources),
		weixinAttempts: make(map[string]weixinAttempt),
		weixinBaseURL:  "https://ilinkai.weixin.qq.com",
		weixinClient:   &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (m *Manager) Run(endpoint string) { go m.runJobs(); go m.runAPI(endpoint) }
func (m *Manager) Close() {
	if m.apiServer != nil {
		_ = m.apiServer.Shutdown(context.Background())
	}
	if m.wdb != nil {
		_ = m.wdb.Close()
	}
}
