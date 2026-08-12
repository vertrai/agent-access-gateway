#!/usr/bin/env python3
"""Fetch a Google access token from Agent Access Gateway."""

import argparse
import json
import os
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


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--token-only", action="store_true")
    args = parser.parse_args()
    base_url = required_env("AGENT_ACCESS_GATEWAY_URL").rstrip("/")
    api_key = required_env("AGENT_ACCESS_GATEWAY_API_KEY")
    request = urllib.request.Request(base_url + "/v1/google-user/access-token")
    request.add_header("Authorization", "Bearer " + api_key)
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            data = json.loads(response.read().decode("utf-8") or "{}")
    except urllib.error.HTTPError as error:
        body = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"gateway returned HTTP {error.code}: {body}") from error
    if not data.get("accessToken"):
        raise RuntimeError("gateway response does not contain an access token")
    print(data["accessToken"] if args.token_only else json.dumps(data, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(json.dumps({"error": str(error)}, ensure_ascii=False), file=sys.stderr)
        sys.exit(1)
