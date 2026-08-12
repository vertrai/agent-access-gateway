---
name: gateway-google-account
description: Fetch the managed Google Workspace account assigned to the current Agent Access Gateway API key. Use when an agent needs its Google email address and password, needs to sign in to Google in a browser, or needs to inspect its assigned Google identity.
---

# Gateway Google Account

Run the bundled helper from this skill directory:

```bash
python3 scripts/google_account.py
```

Configure the process first:

```bash
export AGENT_ACCESS_GATEWAY_URL="https://gateway.example.com"
export AGENT_ACCESS_GATEWAY_API_KEY="gw_sk_..."
```

The same API key always receives the same account. Treat the JSON response as sensitive because it contains the initial password. Never place the password in chat, logs, source control, or command-line arguments.

## Failure boundary

Treat any helper failure as the final result of this request. Report the helper's status and error message to the user, then stop. Do not automatically retry.

Do not obtain or create a Google account through any alternative path. In particular, do not use another Google Account skill, old Access Server or platform-token integrations, browser sign-up, local credentials, Google Admin APIs, or a different service. Do not switch API keys or inspect administrative endpoints to work around the failure.

A failure can mean that the Gateway is temporarily unreachable or that its managed Google account pool is empty. For HTTP 503 or an error indicating that no account is available, tell the user that the server-side account pool may need replenishment and that they can run this same skill again after the pool is replenished. Preserve the Gateway error details, but never include the Gateway API key in the report.

For browser sign-in, acquire the Browser with `gateway-remote-browser`, then fill the returned email and password. Ask the user to take over through the live URL for CAPTCHA, passkey, recovery, 2FA, or other Google security challenges.

Completion criterion: verify that the response contains non-empty `googleUser.email` and `googleUser.password` before using the account.
