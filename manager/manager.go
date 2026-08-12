// Package manager provides the resource-application management service.
// Business workflows intentionally remain empty until their API is specified.
package manager

import (
	"context"
	"net/http"

	"github.com/vertrai/agent-access-gateway/common"
)

var log = common.NewLog("manager")

type Config struct{}

type Manager struct {
	env       string
	config    Config
	apiServer *http.Server
}

func New(env string, config Config) *Manager { return &Manager{env: env, config: config} }

func (m *Manager) Run(endpoint string) { go m.runJobs(); go m.runAPI(endpoint) }

func (m *Manager) Close() {
	if m.apiServer != nil {
		_ = m.apiServer.Shutdown(context.Background())
	}
}
