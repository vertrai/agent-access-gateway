---
name: gateway-google-auth
description: Obtain a short-lived Google OAuth access token for the Google Workspace account assigned to the current Agent Access Gateway API key. Use when an agent or another skill needs authorization for Google APIs such as Gmail or Drive.
---

# Gateway Google Auth

Configure the Gateway URL and API key, then run:

```bash
python3 scripts/google_auth.py
```

Required environment:

```bash
export AGENT_ACCESS_GATEWAY_URL="https://gateway.example.com"
export AGENT_ACCESS_GATEWAY_API_KEY="gw_sk_..."
```

The Gateway caches valid Google tokens and refreshes them near expiry. Request a token when needed instead of maintaining a refresh token in the Agent.

Use `--token-only` only when piping directly into a local process:

```bash
python3 scripts/google_auth.py --token-only
```

Keep tokens out of chat, logs, source control, and command-line arguments. Prefer `gateway-google-workspace` for Gmail and Drive operations so the token is not exposed to the model-facing workflow.

Completion criterion: verify the response has `accessToken`, `email`, and a future `expiresAt` before calling Google APIs.
