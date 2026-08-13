package manager

import (
	"context"
	"fmt"
	"strings"

	"github.com/everFinance/goether"
	"github.com/hymatrix/hymx/sdk"
	"github.com/permadao/goar"
	goarSchema "github.com/permadao/goar/schema"
)

const containerEnvTagPrefix = "Container-Env-"

type HymatrixConfig struct {
	NodeURL, PrivateKey, Module, Scheduler       string
	LLMAPIKey, LLMBaseURL, LLMModel, LLMProvider string
}

type HymatrixClient struct {
	config HymatrixConfig
	sdk    *sdk.SDK
}

func NewHymatrixClient(config HymatrixConfig) (*HymatrixClient, error) {
	if strings.TrimSpace(config.NodeURL) == "" || strings.TrimSpace(config.PrivateKey) == "" || strings.TrimSpace(config.Module) == "" || strings.TrimSpace(config.Scheduler) == "" {
		return nil, fmt.Errorf("hymatrix nodeUrl, privateKey, module and scheduler are required")
	}
	signer, err := goether.NewSigner(config.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("create hymatrix signer: %w", err)
	}
	bundler, err := goar.NewBundler(signer)
	if err != nil {
		return nil, fmt.Errorf("create hymatrix bundler: %w", err)
	}
	return &HymatrixClient{config: config, sdk: sdk.NewFromBundler(config.NodeURL, bundler)}, nil
}

func (h *HymatrixClient) Spawn(_ context.Context, runtimeType string) (string, error) {
	if strings.TrimSpace(runtimeType) == "" {
		return "", fmt.Errorf("runtimeType is required")
	}
	res, err := h.sdk.SpawnAndWait(h.config.Module, h.config.Scheduler, []goarSchema.Tag{{Name: containerEnvTagPrefix + "RUNTIME_TYPE", Value: runtimeType}})
	if err != nil {
		return "", err
	}
	return res.Id, nil
}
func (h *HymatrixClient) Start(_ context.Context, in PodStartInput) error {
	if strings.TrimSpace(in.PID) == "" {
		return fmt.Errorf("hymatrix pid is required")
	}
	provider := strings.TrimSpace(h.config.LLMProvider)
	if provider == "" {
		provider = "custom"
	}
	params := []goarSchema.Tag{{Name: "Action", Value: "start"}, {Name: "LLM_BASE_URL", Value: h.config.LLMBaseURL}, {Name: "LLM_MODEL", Value: h.config.LLMModel}, {Name: "LLM_PROVIDER", Value: provider}, {Name: "AGENT_ACCESS_GATEWAY_URL", Value: in.GatewayURL}, {Name: "ACCESS_SERVER_URL", Value: in.GatewayURL}}
	secrets := []goarSchema.Tag{{Name: "AGENT_ACCESS_GATEWAY_API_KEY", Value: in.GatewayAPIKey}, {Name: "BROWSER_API_KEY", Value: in.GatewayAPIKey}, {Name: "LLM_API_KEY", Value: h.config.LLMAPIKey}}
	if in.BotToken != "" {
		secrets = append(secrets, goarSchema.Tag{Name: "Bot_Token", Value: in.BotToken})
	}
	_, err := h.sdk.SendMessageWithEncryptedParamsAndWait(in.PID, "", params, secrets)
	return err
}
func (h *HymatrixClient) Stop(_ context.Context, pid string) error {
	if strings.TrimSpace(pid) == "" {
		return fmt.Errorf("hymatrix pid is required")
	}
	_, err := h.sdk.SendMessageAndWait(pid, "", []goarSchema.Tag{{Name: "Action", Value: "stop"}})
	return err
}

type PodStartInput struct{ PID, GatewayURL, GatewayAPIKey, BotToken string }
