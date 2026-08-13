package manager

import "testing"

func TestBuildPodSpawnTagsIncludesCompleteEnvironment(t *testing.T) {
	config := HymatrixConfig{
		LLMProvider: "custom",
		LLMModel:    "deepseek-chat",
		LLMBaseURL:  "https://llm.example/v1",
		LLMAPIKey:   "llm-secret",
	}
	tags, err := buildPodSpawnTags(config, PodSpawnInput{
		RuntimeType:   "hermes",
		GatewayURL:    "https://hub.example",
		GatewayAPIKey: "gw_sk_test",
		BotToken:      "telegram-token",
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
		"Container-Env-AGENT_ACCESS_GATEWAY_URL":        "https://hub.example",
		"Container-Env-AGENT_ACCESS_GATEWAY_API_KEY":    "gw_sk_test",
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
	tags, err := buildPodSpawnTags(HymatrixConfig{}, PodSpawnInput{RuntimeType: "hermes", GatewayURL: "https://hub.example", GatewayAPIKey: "gw_sk_test"})
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
