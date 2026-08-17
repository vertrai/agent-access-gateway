package starthermes

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

//go:embed all:tools/browser-harness
var embeddedBrowserHarness embed.FS

func Run() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}
	config, err := GatewayConfigFromEnv()
	if err != nil {
		return err
	}
	if err := InstallSkills(home); err != nil {
		return err
	}
	if err := ensureBrowserHarness(); err != nil {
		return err
	}
	if err := WriteHermesGatewayEnv(home, config); err != nil {
		return fmt.Errorf("write Hermes gateway environment: %w", err)
	}
	hermes, err := hermesExecutable(home)
	if err != nil {
		return err
	}
	if err := configureHermes(hermes); err != nil {
		return err
	}
	if envFirst("HERMES_AGENT_TELEGRAM_BOT_TOKEN", "TELEGRAM_BOT_TOKEN", "Bot_Token") != "" {
		if err := ConfigureTelegramAutoHomeChannel(home, hermes); err != nil {
			return fmt.Errorf("configure Telegram auto home channel: %w", err)
		}
	}
	args := []string{hermes, "gateway", "run", "-q", "--replace", "--accept-hooks"}
	if err := syscall.Exec(hermes, args, os.Environ()); err != nil {
		return fmt.Errorf("exec Hermes gateway: %w", err)
	}
	return nil
}

func ensureBrowserHarness() error {
	uv, err := exec.LookPath("uv")
	if err != nil {
		return fmt.Errorf("browser-harness is missing and uv is unavailable: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	installDirectory := filepath.Join(home, ".harness", "browser-harness")
	if err := extractBrowserHarness(installDirectory); err != nil {
		return fmt.Errorf("extract embedded browser-harness: %w", err)
	}
	command := exec.Command(uv, "tool", "install", "-e", ".")
	command.Dir = installDirectory
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("install browser-harness: %w: %s", err, string(output))
	}
	python := filepath.Join(home, ".local", "share", "uv", "tools", "browser-harness", "bin", "python")
	verify := exec.Command(python, "-c", "import browser_harness._ipc; import browser_harness.admin")
	if output, err := verify.CombinedOutput(); err != nil {
		return fmt.Errorf("verify browser-harness: %w: %s", err, string(output))
	}
	return nil
}

func extractBrowserHarness(destinationRoot string) error {
	const sourceRoot = "tools/browser-harness"
	if err := os.RemoveAll(destinationRoot); err != nil {
		return err
	}
	return fs.WalkDir(embeddedBrowserHarness, sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative := strings.TrimPrefix(strings.TrimPrefix(path, sourceRoot), "/")
		destination := filepath.Join(destinationRoot, filepath.FromSlash(relative))
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		content, err := embeddedBrowserHarness.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, content, 0o644)
	})
}

func configureHermes(hermes string) error {
	provider := envFirst("HERMES_AGENT_LLM_PROVIDER", "LLM_PROVIDER")
	model := envFirst("HERMES_AGENT_LLM_MODEL", "LLM_MODEL")
	baseURL := envFirst("HERMES_AGENT_LLM_BASE_URL", "LLM_BASE_URL")
	apiKey := envFirst("HERMES_AGENT_LLM_API_KEY", "LLM_API_KEY")
	settings := [][2]string{
		{"approvals.mode", "off"},
		{"hooks_auto_accept", "true"},
		{"approvals.mcp_reload_confirm", "false"},
	}
	if provider != "" {
		settings = append(settings, [2]string{"model.provider", provider})
	}
	if model != "" {
		settings = append(settings, [2]string{"model.default", model})
	}
	if baseURL != "" {
		settings = append(settings, [2]string{"model.base_url", baseURL})
	}
	if apiKey != "" {
		settings = append(settings, [2]string{"model.api_key", apiKey})
	}
	for _, setting := range settings {
		command := exec.Command(hermes, "config", "set", setting[0], setting[1])
		if output, err := command.CombinedOutput(); err != nil {
			if setting[0] == "model.api_key" {
				return fmt.Errorf("configure Hermes %s: %w (output redacted)", setting[0], err)
			}
			return fmt.Errorf("configure Hermes %s: %w: %s", setting[0], err, string(output))
		}
	}
	return nil
}

func hermesExecutable(home string) (string, error) {
	if path, err := exec.LookPath("hermes"); err == nil {
		return path, nil
	}
	for _, path := range []string{
		filepath.Join(home, ".local", "bin", "hermes"),
		filepath.Join(home, ".hermes", "hermes-agent", "venv", "bin", "hermes"),
		"/usr/local/bin/hermes",
	} {
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Mode().Perm()&0o111 != 0 {
			return path, nil
		}
	}
	return "", fmt.Errorf("hermes executable not found")
}
