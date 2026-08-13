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

func (h *HymatrixClient) Spawn(_ context.Context, in PodSpawnInput) (string, error) {
	tags, err := buildPodSpawnTags(h.config, in)
	if err != nil {
		return "", err
	}
	res, err := h.sdk.SpawnAndWait(h.config.Module, h.config.Scheduler, tags)
	if err != nil {
		return "", err
	}
	return res.Id, nil
}

func buildPodSpawnTags(config HymatrixConfig, in PodSpawnInput) ([]goarSchema.Tag, error) {
	if strings.TrimSpace(in.RuntimeType) == "" {
		return nil, fmt.Errorf("runtimeType is required")
	}
	if strings.TrimSpace(in.GatewayURL) == "" || strings.TrimSpace(in.GatewayAPIKey) == "" {
		return nil, fmt.Errorf("gateway URL and API key are required")
	}
	provider := strings.TrimSpace(config.LLMProvider)
	if provider == "" {
		provider = "custom"
	}
	values := [][2]string{
		{"RUNTIME_TYPE", in.RuntimeType},
		{"HERMES_AGENT_LLM_PROVIDER", provider},
		{"HERMES_AGENT_LLM_MODEL", config.LLMModel},
		{"HERMES_AGENT_LLM_BASE_URL", config.LLMBaseURL},
		{"HERMES_AGENT_LLM_API_KEY", config.LLMAPIKey},
		{"HUB_GATEWAY_URL", in.GatewayURL},
		{"HUB_GATEWAY_API_KEY", in.GatewayAPIKey},
		{"AGENT_ACCESS_GATEWAY_URL", in.GatewayURL},
		{"AGENT_ACCESS_GATEWAY_API_KEY", in.GatewayAPIKey},
	}
	if in.BotToken != "" {
		values = append(values, [2]string{"HERMES_AGENT_TELEGRAM_BOT_TOKEN", in.BotToken})
	}
	tags := make([]goarSchema.Tag, 0, len(values))
	for _, value := range values {
		if value[1] != "" {
			tags = append(tags, goarSchema.Tag{Name: containerEnvTagPrefix + value[0], Value: value[1]})
		}
	}
	return tags, nil
}

type PodSpawnInput struct{ RuntimeType, GatewayURL, GatewayAPIKey, BotToken string }
