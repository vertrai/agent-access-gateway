package starthermes

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const telegramAutoHomePluginName = "telegram-auto-home-channel"

// ConfigureTelegramAutoHomeChannel installs a Hermes hook that silently makes
// the first Telegram DM the home channel before Hermes emits its /sethome hint.
func ConfigureTelegramAutoHomeChannel(home, hermes string) error {
	pluginDirectory := filepath.Join(home, ".hermes", "plugins", telegramAutoHomePluginName)
	if err := os.MkdirAll(pluginDirectory, 0o700); err != nil {
		return fmt.Errorf("create plugin directory: %w", err)
	}
	files := map[string]string{
		"plugin.yaml": telegramAutoHomePluginManifest(),
		"__init__.py": telegramAutoHomePluginCode(),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(pluginDirectory, name), []byte(content), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	command := exec.Command(hermes, "plugins", "enable", telegramAutoHomePluginName)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("enable plugin: %w: %s", err, string(output))
	}
	return nil
}

func telegramAutoHomePluginManifest() string {
	return `name: telegram-auto-home-channel
version: "1.0.0"
description: Automatically sets the first Telegram DM as the Hermes home channel before the runtime /sethome hint is emitted.
hooks:
  - pre_gateway_dispatch
`
}

func telegramAutoHomePluginCode() string {
	return `"""Automatically designate the first Telegram DM as the home channel."""

import os

HOME_ENV = "TELEGRAM_HOME_CHANNEL"
HOME_NAME_ENV = "TELEGRAM_HOME_CHANNEL_NAME"
HOME_THREAD_ENV = "TELEGRAM_HOME_CHANNEL_THREAD_ID"


def _source_value(source, attr, default=""):
    value = getattr(source, attr, default)
    value = getattr(value, "value", value)
    return "" if value is None else str(value).strip()


def _save_env(key, value):
    from hermes_cli.config import save_env_value

    save_env_value(key, value)
    os.environ[key] = value


def _sync_gateway_config(gateway, source, chat_id, chat_name, thread_id):
    try:
        from gateway.config import HomeChannel, Platform, PlatformConfig
    except Exception:
        return

    platform = getattr(source, "platform", None)
    if platform is None:
        try:
            platform = Platform.TELEGRAM
        except Exception:
            return

    try:
        platform_config = gateway.config.platforms.setdefault(
            platform,
            PlatformConfig(enabled=True),
        )
        platform_config.home_channel = HomeChannel(
            platform=platform,
            chat_id=chat_id,
            name=chat_name,
            thread_id=thread_id or None,
        )
    except Exception:
        return


def auto_set_telegram_home(event, gateway=None, **kwargs):
    source = getattr(event, "source", None)
    if source is None:
        return None

    platform = _source_value(source, "platform")
    if platform and platform != "telegram":
        return None

    # Bind DMs only by default so a group cannot accidentally become the
    # destination for cron results and cross-platform messages.
    chat_type = _source_value(source, "chat_type")
    allow_groups = os.getenv("HERMES_AUTO_TELEGRAM_HOME_ALLOW_GROUPS", "").lower() in {
        "1",
        "true",
        "yes",
        "on",
    }
    if chat_type and chat_type != "dm" and not allow_groups:
        return None

    chat_id = _source_value(source, "chat_id")
    if not chat_id or os.getenv(HOME_ENV, "").strip():
        return None

    chat_name = _source_value(source, "chat_name") or _source_value(source, "user_name") or "Telegram Home"
    thread_id = _source_value(source, "thread_id")

    try:
        _save_env(HOME_ENV, chat_id)
        _save_env(HOME_NAME_ENV, chat_name)
        _save_env(HOME_THREAD_ENV, thread_id)
        if gateway is not None:
            _sync_gateway_config(gateway, source, chat_id, chat_name, thread_id)
    except Exception:
        return None

    return None


def register(ctx):
    ctx.register_hook("pre_gateway_dispatch", auto_set_telegram_home)
`
}
