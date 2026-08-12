---
name: gateway-remote-browser
description: Acquire and manage the persistent remote browser assigned to the current Agent Access Gateway API key. Use when an agent needs a CDP endpoint, a live browser URL, a fresh browser session with the same profile, or needs to close its remote session.
---

# Gateway Remote Browser

This skill requires the `browser-harness` CLI and daemon runtime. The repository installer installs it with `uv tool install browser-harness`. Before use, verify:

```bash
browser-harness --version
```

Configure the Gateway once:

```bash
export AGENT_ACCESS_GATEWAY_URL="https://gateway.example.com"
export AGENT_ACCESS_GATEWAY_API_KEY="gw_sk_..."
```

Connect before every browser task:

```bash
python3 scripts/browser_connect.py
```

Always use this command, even when `browser-harness` was installed by uv tool. Do not locate or invoke the uv tool environment's Python manually. The connector detects the `browser-harness` installation and re-executes itself with the correct tool Python when required.

The connector obtains the API key's Browser and attaches a named browser-harness daemon to its CDP endpoint. If attachment fails, it resets the Gateway Session exactly once, restarts the named daemon, and attaches to the replacement. Its output is:

```text
BU_NAME=gateway-brw_xxx
BU_CDP_URL=wss://...
BU_LIVE_URL=https://...
```

Use the returned `BU_NAME` for every browser operation:

```bash
BU_NAME=<returned-name> browser-harness -c 'print(page_info())'
```

For multi-step work:

```bash
BU_NAME=<returned-name> browser-harness <<'PY'
new_tab("https://example.com")
wait_for_load()
print(page_info())
PY
```

Use the resource helper only for explicit lifecycle operations:

```bash
python3 scripts/remote_browser.py get
python3 scripts/remote_browser.py reset
python3 scripts/remote_browser.py close
```

`get` creates or reuses the API key's Browser and returns `cdpUrl` and `liveUrl`. Prefer `browser_connect.py` because it also attaches browser-harness.

If the connector reports that the browser-harness runtime was not found, treat the Skill installation as incomplete. Run the repository Skill installer again and retry the connector once. Do not replace `browser_connect.py` with an ad hoc invocation of browser-harness internals.

Use `reset` when the current Session is stale or unusable. Reset stops the Session and starts another one with the same provider profile, preserving its login state. Retry the original browser action once after reset.

Use `close` after work that should not leave a paid Session running. Closing preserves the profile; the next `get` starts a new Session with that profile. Closing is idempotent.

Whenever `liveUrl` is available, share it with the user when manual login, CAPTCHA, verification, or review is required. Never invent a live URL.

Completion criterion: before automation, verify that the connector returned `BU_NAME` and that a `BU_NAME=<name> browser-harness` inspection succeeds; after reset, verify the replacement daemon is attached; after close, verify the `closed` result.
