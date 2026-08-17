package manager

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestFetchHymatrixNodeInfoUsesInfoEndpoint(t *testing.T) {
	previous := nodeInfoHTTPClient
	t.Cleanup(func() { nodeInfoHTTPClient = previous })
	nodeInfoHTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://1.1.1.1/node/info" {
			t.Fatalf("request URL = %q", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body: io.NopCloser(strings.NewReader(`{
				"Protocol":"hymx",
				"Node-Version":"v0.4.8",
				"Node":{"Acc-Id":"0x67cBa2FEDaaA07627169a60Bc690aD9571Ed2265","Name":"node"}
			}`)),
		}, nil
	})}

	info, err := fetchHymatrixNodeInfo(context.Background(), "https://1.1.1.1/node/?ignored=yes")
	if err != nil {
		t.Fatal(err)
	}
	if info.Node.AccountID != "0x67cBa2FEDaaA07627169a60Bc690aD9571Ed2265" {
		t.Fatalf("scheduler = %q", info.Node.AccountID)
	}
}

func TestFetchHymatrixNodeInfoRejectsPrivateAddress(t *testing.T) {
	_, err := fetchHymatrixNodeInfo(context.Background(), "http://127.0.0.1:8081")
	if err == nil || !strings.Contains(err.Error(), "private or local") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildPodSpawnTagsIncludesCompleteEnvironment(t *testing.T) {
	config := HymatrixConfig{
		LLMProvider: "custom",
		LLMModel:    "deepseek-chat",
		LLMBaseURL:  "https://llm.example/v1",
		LLMAPIKey:   "llm-secret",
	}
	tags, err := buildPodSpawnTags(config, PodSpawnInput{
		RuntimeType:        "hermes",
		GatewayURL:         "https://hub.example",
		GatewayAPIKey:      "gw_sk_test",
		BotToken:           "telegram-token",
		HermesGatewayToken: "hermes-health-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"Container-Env-RUNTIME_TYPE":                    "hermes",
		"Container-Env-HERMES_AGENT_LLM_PROVIDER":       "custom",
		"Container-Env-HERMES_AGENT_LLM_MODEL":          "deepseek-chat",
		"Container-Env-HERMES_AGENT_LLM_BASE_URL":       "https://llm.example/v1",
		"Container-Env-HERMES_AGENT_LLM_API_KEY":        "llm-secret",
		"Container-Env-HUB_GATEWAY_URL":                 "https://hub.example",
		"Container-Env-HUB_GATEWAY_API_KEY":             "gw_sk_test",
		"Container-Env-API_SERVER_ENABLED":              "true",
		"Container-Env-API_SERVER_KEY":                  "hermes-health-secret",
		"Container-Env-HERMES_GATEWAY_TOKEN":            "hermes-health-secret",
		"Container-Env-HERMES_AGENT_TELEGRAM_BOT_TOKEN": "telegram-token",
	}
	if len(tags) != len(want) {
		t.Fatalf("tag count = %d, want %d", len(tags), len(want))
	}
	for _, tag := range tags {
		value, ok := want[tag.Name]
		if !ok {
			t.Fatalf("unexpected tag %q", tag.Name)
		}
		if tag.Value != value {
			t.Errorf("tag %s = %q, want %q", tag.Name, tag.Value, value)
		}
		delete(want, tag.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing tags: %v", want)
	}
}

func TestBuildPodSpawnTagsDefaultsProviderAndOmitsEmptyValues(t *testing.T) {
	tags, err := buildPodSpawnTags(HymatrixConfig{}, PodSpawnInput{RuntimeType: "hermes", GatewayURL: "https://hub.example", GatewayAPIKey: "gw_sk_test", HermesGatewayToken: "hermes-health-secret"})
	if err != nil {
		t.Fatal(err)
	}
	values := make(map[string]string, len(tags))
	for _, tag := range tags {
		values[tag.Name] = tag.Value
	}
	if values["Container-Env-HERMES_AGENT_LLM_PROVIDER"] != "custom" {
		t.Fatalf("provider = %q", values["Container-Env-HERMES_AGENT_LLM_PROVIDER"])
	}
	if _, exists := values["Container-Env-HERMES_AGENT_LLM_API_KEY"]; exists {
		t.Fatal("empty LLM API key should not be emitted")
	}
}

func TestBuildPodSpawnTagsRequiresHermesGatewayToken(t *testing.T) {
	_, err := buildPodSpawnTags(HymatrixConfig{}, PodSpawnInput{RuntimeType: "hermes", GatewayURL: "https://hub.example", GatewayAPIKey: "gw_sk_test"})
	if err == nil || !strings.Contains(err.Error(), "Hermes gateway token") {
		t.Fatalf("error = %v", err)
	}
}
