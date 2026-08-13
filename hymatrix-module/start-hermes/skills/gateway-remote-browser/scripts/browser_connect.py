#!/usr/bin/env python3
"""Attach browser-harness to a Hub Gateway remote Browser."""

import json
import os
import shlex
import shutil
import subprocess
import sys
import urllib.error
import urllib.request
from pathlib import Path


def configured_value(name):
    return os.environ.get(name, "").strip() or hermes_env_value(name)


def gateway_credentials():
    current = configured_value("HUB_GATEWAY_URL"), configured_value("HUB_GATEWAY_API_KEY")
    if all(current):
        return current
    legacy = configured_value("AGENT_ACCESS_GATEWAY_URL"), configured_value("AGENT_ACCESS_GATEWAY_API_KEY")
    if all(legacy):
        return legacy
    raise RuntimeError("HUB_GATEWAY_URL and HUB_GATEWAY_API_KEY are required as a complete pair")


def hermes_env_value(name):
    path = Path(os.environ.get("HERMES_HOME", str(Path.home() / ".hermes"))) / ".env"
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError:
        return ""
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("export "):
            stripped = stripped[7:].lstrip()
        if stripped.startswith(name + "="):
            return stripped.split("=", 1)[1].strip().strip('"\'')
    return ""


def gateway_browser(reset=False):
    base_url, api_key = gateway_credentials()
    base_url = base_url.rstrip("/")
    path = "/v1/browser/reset" if reset else "/v1/browser"
    request = urllib.request.Request(base_url + path, method="POST" if reset else "GET")
    request.add_header("Authorization", "Bearer " + api_key)
    try:
        with urllib.request.urlopen(request, timeout=90) as response:
            data = json.loads(response.read().decode("utf-8") or "{}")
    except urllib.error.HTTPError as error:
        body = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"gateway returned HTTP {error.code}: {body}") from error
    browser = data.get("browser") or {}
    if not browser.get("id") or not browser.get("cdpUrl"):
        raise RuntimeError("gateway response does not contain browser.id and browser.cdpUrl")
    return browser


def harness_python_candidates():
    configured = os.environ.get("BROWSER_HARNESS_PYTHON", "").strip()
    if configured:
        yield configured

    # uv links the browser-harness entry point into its tool bin directory. The
    # resolved entry point lives beside the tool environment's Python, which is
    # more reliable than assuming the default UV_TOOL_DIR.
    harness = shutil.which("browser-harness")
    if harness:
        resolved_harness = Path(harness).expanduser().resolve()
        yield str(resolved_harness.parent / "python")

    uv = shutil.which("uv")
    if uv:
        try:
            tool_dir = subprocess.check_output([uv, "tool", "dir"], text=True, timeout=10).strip()
            if tool_dir:
                yield str(Path(tool_dir) / "browser-harness" / "bin" / "python")
        except Exception:
            pass
    yield str(Path.home() / ".local" / "share" / "uv" / "tools" / "browser-harness" / "bin" / "python")


def ensure_harness_runtime():
    try:
        import browser_harness.admin  # noqa: F401
        return
    except ImportError:
        pass
    current = Path(sys.executable).resolve()
    for candidate in harness_python_candidates():
        path = Path(candidate).expanduser()
        if path.is_file() and path.resolve() != current:
            os.execv(str(path), [str(path), str(Path(__file__).resolve()), *sys.argv[1:]])
    raise RuntimeError(
        "browser-harness runtime was not found; run the Hub Gateway skill installer again"
    )


def daemon_name(browser):
    safe_id = "".join(character if character.isalnum() or character in "-_" else "-" for character in browser["id"])
    return "gateway-" + safe_id


def attach(browser, restart=False):
    import browser_harness.admin as admin

    name = daemon_name(browser)
    if restart:
        try:
            admin.restart_daemon(name=name)
        except Exception:
            pass
    cdp_url = browser["cdpUrl"]
    remote_env = {"BU_CDP_WS": cdp_url} if cdp_url.startswith(("ws://", "wss://")) else {"BU_CDP_URL": cdp_url}
    admin.ensure_daemon(wait=20, name=name, env=remote_env)
    return name


def print_connection(name, browser, reset):
    print(f"BU_NAME={shlex.quote(name)}")
    print(f"BU_CDP_URL={shlex.quote(browser['cdpUrl'])}")
    if browser.get("liveUrl"):
        print(f"BU_LIVE_URL={shlex.quote(browser['liveUrl'])}")
    print(f"BU_RESET={'true' if reset else 'false'}")


def main():
    ensure_harness_runtime()
    browser = gateway_browser(reset=False)
    try:
        name = attach(browser)
        print_connection(name, browser, False)
        return
    except Exception as first_error:
        print(f"STAGE browser attach failed; resetting once: {first_error}", file=sys.stderr)
    browser = gateway_browser(reset=True)
    name = attach(browser, restart=True)
    print_connection(name, browser, True)


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(json.dumps({"error": str(error)}, ensure_ascii=False), file=sys.stderr)
        sys.exit(1)
