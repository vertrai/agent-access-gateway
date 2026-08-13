package starthermes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGatewayConfigFromEnvUsesCompletePair(t *testing.T) {
	t.Setenv("HUB_GATEWAY_URL", "https://hub.example")
	t.Setenv("HUB_GATEWAY_API_KEY", "hub-key")
	t.Setenv("AGENT_ACCESS_GATEWAY_URL", "https://legacy.example")
	t.Setenv("AGENT_ACCESS_GATEWAY_API_KEY", "legacy-key")
	config, err := GatewayConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if config.URL != "https://hub.example" || config.APIKey != "hub-key" {
		t.Fatalf("unexpected config: %#v", config)
	}
}

func TestGatewayConfigFromEnvFallsBackAsPair(t *testing.T) {
	t.Setenv("HUB_GATEWAY_URL", "https://partial.example")
	t.Setenv("HUB_GATEWAY_API_KEY", "")
	t.Setenv("AGENT_ACCESS_GATEWAY_URL", "https://legacy.example")
	t.Setenv("AGENT_ACCESS_GATEWAY_API_KEY", "legacy-key")
	config, err := GatewayConfigFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if config.URL != "https://legacy.example" || config.APIKey != "legacy-key" {
		t.Fatalf("mixed credential pair: %#v", config)
	}
}

func TestInstallSkills(t *testing.T) {
	home := t.TempDir()
	if err := InstallSkills(home); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gateway-google-account", "gateway-google-auth", "gateway-google-workspace", "gateway-remote-browser"} {
		if _, err := os.Stat(filepath.Join(home, ".hermes", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("skill %s was not installed: %v", name, err)
		}
	}
}

func TestWriteHermesGatewayEnvPreservesUnrelatedValues(t *testing.T) {
	home := t.TempDir()
	directory := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, ".env")
	if err := os.WriteFile(path, []byte("KEEP_ME=yes\nHUB_GATEWAY_URL=old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteHermesGatewayEnv(home, GatewayConfig{URL: "https://hub.example", APIKey: "gw_sk_test"}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	for _, expected := range []string{"KEEP_ME=yes", "HUB_GATEWAY_URL=https://hub.example", "HUB_GATEWAY_API_KEY=gw_sk_test"} {
		if !containsLine(text, expected) {
			t.Errorf("missing %q in %q", expected, text)
		}
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("environment file mode = %v, err = %v", info.Mode().Perm(), err)
	}
}

func TestWriteHermesGatewayEnvRejectsNewlines(t *testing.T) {
	home := t.TempDir()
	err := WriteHermesGatewayEnv(home, GatewayConfig{URL: "https://hub.example\nINJECTED=yes", APIKey: "gw_sk_test"})
	if err == nil {
		t.Fatal("expected newline validation error")
	}
}

func containsLine(content, expected string) bool {
	for _, line := range splitLines(content) {
		if line == expected {
			return true
		}
	}
	return false
}

func splitLines(content string) []string {
	var lines []string
	start := 0
	for index, value := range content {
		if value == '\n' {
			lines = append(lines, content[start:index])
			start = index + 1
		}
	}
	return lines
}
