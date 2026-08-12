#!/usr/bin/env python3
"""Acquire, reset, or close an Agent Access Gateway remote browser."""

import argparse
import json
import os
import shlex
import sys
import urllib.error
import urllib.request
from pathlib import Path


def required_env(name):
    value = os.environ.get(name, "").strip()
    if not value:
        value = hermes_env_value(name)
    if not value:
        raise RuntimeError(f"{name} is required")
    return value


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


def gateway_request(command):
    base_url = required_env("AGENT_ACCESS_GATEWAY_URL").rstrip("/")
    api_key = required_env("AGENT_ACCESS_GATEWAY_API_KEY")
    path, method = {
        "get": ("/v1/browser", "GET"),
        "reset": ("/v1/browser/reset", "POST"),
        "close": ("/v1/browser/close", "POST"),
    }[command]
    request = urllib.request.Request(base_url + path, method=method)
    request.add_header("Authorization", "Bearer " + api_key)
    try:
        with urllib.request.urlopen(request, timeout=90) as response:
            return json.loads(response.read().decode("utf-8") or "{}")
    except urllib.error.HTTPError as error:
        body = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"gateway returned HTTP {error.code}: {body}") from error


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("command", choices=("get", "reset", "close"))
    parser.add_argument("--format", choices=("json", "env"), default="json")
    args = parser.parse_args()
    data = gateway_request(args.command)
    if args.command != "close":
        browser = data.get("browser") or {}
        if not browser.get("cdpUrl"):
            raise RuntimeError("gateway response does not contain browser.cdpUrl")
    if args.format == "env":
        browser = data.get("browser") or {}
        values = {
            "AGENT_BROWSER_ID": browser.get("id", ""),
            "AGENT_BROWSER_CDP_URL": browser.get("cdpUrl", ""),
            "AGENT_BROWSER_LIVE_URL": browser.get("liveUrl", ""),
        }
        for key, value in values.items():
            print(f"{key}={shlex.quote(str(value))}")
        return
    print(json.dumps(data, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(json.dumps({"error": str(error)}, ensure_ascii=False), file=sys.stderr)
        sys.exit(1)
