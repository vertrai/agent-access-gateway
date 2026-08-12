---
name: gateway-google-workspace
description: Operate Gmail and Google Drive as the managed Google Workspace account assigned to the current Agent Access Gateway API key. Use when an agent needs to read or send email, inspect mailbox messages, list or create Drive content, upload or download files, or manage Drive resources.
---

# Gateway Google Workspace

Use the bundled helper for Gmail and Drive operations. It obtains a short-lived token from Agent Access Gateway for every invocation and does not persist the token.

Configure:

```bash
export AGENT_ACCESS_GATEWAY_URL="https://gateway.example.com"
export AGENT_ACCESS_GATEWAY_API_KEY="gw_sk_..."
```

Inspect available commands:

```bash
python3 scripts/google_workspace.py --help
python3 scripts/google_workspace.py gmail-list --help
python3 scripts/google_workspace.py drive-list --help
```

Read [references/commands.md](references/commands.md) for command examples and response behavior.

For read operations, run the command directly. Before sending email, deleting content, changing sharing, or performing another externally visible action, obtain the user's confirmation unless their request already explicitly authorizes that exact action.

Return useful identifiers and web links from the JSON result. Treat message bodies, account data, and downloaded files as user data. Keep the Gateway API key and Google access token out of chat, logs, source control, and command-line arguments.

If Google returns `401`, rerun the command once so the Gateway can issue a fresh token. If Google returns `403`, report the missing scope or Workspace policy; do not start an interactive OAuth flow because this Gateway uses Domain-Wide Delegation.

Completion criterion: verify the JSON response represents the requested Gmail or Drive state, including IDs for mutations and the output path for downloads.
