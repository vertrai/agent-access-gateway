#!/usr/bin/env python3
"""Persist Agent Access Gateway settings for a supported Agent host."""

import argparse
import getpass
import os
import shutil
import stat
import subprocess
import sys
import tempfile
from pathlib import Path


KEYS = ("AGENT_ACCESS_GATEWAY_URL", "AGENT_ACCESS_GATEWAY_API_KEY")


def hermes_env_path():
    hermes = shutil.which("hermes")
    if hermes:
        try:
            value = subprocess.check_output([hermes, "config", "env-path"], text=True, timeout=10).strip()
            if value:
                return Path(value).expanduser()
        except Exception:
            pass
    return Path(os.environ.get("HERMES_HOME", str(Path.home() / ".hermes"))) / ".env"


def quoted(value):
    return '"' + value.replace("\\", "\\\\").replace('"', '\\"') + '"'


def update_env_file(path, values):
    path.parent.mkdir(parents=True, exist_ok=True)
    original = path.read_text(encoding="utf-8").splitlines() if path.exists() else []
    output = []
    replaced = set()
    for line in original:
        stripped = line.strip()
        matched = False
        for key in KEYS:
            if stripped.startswith(key + "=") or stripped.startswith("export " + key + "="):
                output.append(f"{key}={quoted(values[key])}")
                replaced.add(key)
                matched = True
                break
        if not matched:
            output.append(line)
    if output and output[-1] != "":
        output.append("")
    for key in KEYS:
        if key not in replaced:
            output.append(f"{key}={quoted(values[key])}")
    content = "\n".join(output) + "\n"
    descriptor, temporary = tempfile.mkstemp(prefix=".agent-access-gateway-", dir=str(path.parent))
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as handle:
            handle.write(content)
        os.chmod(temporary, stat.S_IRUSR | stat.S_IWUSR)
        os.replace(temporary, path)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)
    os.chmod(path, stat.S_IRUSR | stat.S_IWUSR)


def main():
    parser = argparse.ArgumentParser(description="Configure Agent Access Gateway for an Agent host")
    parser.add_argument("--agent", choices=("hermes",), required=True)
    parser.add_argument("--gateway-url", required=True)
    parser.add_argument("--env-file", default="", help=argparse.SUPPRESS)
    args = parser.parse_args()
    gateway_url = args.gateway_url.strip().rstrip("/")
    if not gateway_url.startswith(("http://", "https://")):
        raise RuntimeError("gateway URL must start with http:// or https://")
    api_key = os.environ.pop("AGENT_ACCESS_GATEWAY_API_KEY_INPUT", "").strip()
    if not api_key and sys.stdin.isatty():
        api_key = getpass.getpass("Agent Access Gateway API Key: ").strip()
    if not api_key:
        raise RuntimeError("provide the API key through hidden input or AGENT_ACCESS_GATEWAY_API_KEY_INPUT")
    if not api_key.startswith("gw_sk_"):
        raise RuntimeError("gateway API key must start with gw_sk_")
    path = Path(args.env_file).expanduser() if args.env_file else hermes_env_path()
    update_env_file(path, {"AGENT_ACCESS_GATEWAY_URL": gateway_url, "AGENT_ACCESS_GATEWAY_API_KEY": api_key})
    print(f"Configured Agent Access Gateway in {path}")
    print("The API key was stored with file mode 0600 and was not printed.")


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print(f"ERROR: {error}", file=sys.stderr)
        sys.exit(1)
