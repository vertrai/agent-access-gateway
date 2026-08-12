# Install Agent Access Gateway Skills

This document is an executable runbook for Codex, Claude Code, Hermes, and other agents that support `SKILL.md` directories.

## Instructions for the Agent

Perform every step. Ask the user only when credentials or execution approval are required.

1. Identify the host as `codex`, `claude`, or `hermes`. For another host, determine its absolute skills directory.
2. Verify that `git`, `sh`, `python3`, and `uv` are available. If `uv` is missing, install it from the official Astral instructions at <https://docs.astral.sh/uv/getting-started/installation/>. Request execution approval if the host requires it.
3. Resolve the source directory. If this document was provided from an existing local checkout containing `scripts/install-skills.sh` and `skills/`, use that checkout and skip cloning. Otherwise create a temporary directory and use sparse checkout to download only the Skill payload (`skills/` and `scripts/`), not the Go server source:

   ```bash
   git clone --depth 1 --filter=blob:none --sparse \
     https://github.com/vertrai/agent-access-gateway.git \
     <temporary-directory>/agent-access-gateway
   git -C <temporary-directory>/agent-access-gateway \
     sparse-checkout set --no-cone '/skills/' '/scripts/'
   ```

   Verify that `<temporary-directory>/agent-access-gateway/skills` and `<temporary-directory>/agent-access-gateway/scripts/install-skills.sh` exist. Do not expand the sparse checkout to download the Go application.

4. Install for the detected host. Replace `<source-directory>` with the local checkout or cloned directory resolved above:

   ```bash
   <source-directory>/scripts/install-skills.sh --agent codex
   <source-directory>/scripts/install-skills.sh --agent claude
   <source-directory>/scripts/install-skills.sh --agent hermes
   ```

   Run exactly one command. For another host, pass its absolute skills directory instead:

   ```bash
   <source-directory>/scripts/install-skills.sh /absolute/path/to/skills
   ```

5. Confirm these four files exist in the destination:

   - `gateway-google-account/SKILL.md`
   - `gateway-google-auth/SKILL.md`
   - `gateway-google-workspace/SKILL.md`
   - `gateway-remote-browser/SKILL.md`

   Also run `browser-harness --version`. The installer installs this required Remote Browser dependency through `uv tool install browser-harness` when missing. Installation is incomplete until this command succeeds.

6. Configure the Gateway connection.

   For Hermes, check `AGENT_ACCESS_GATEWAY_URL` and `AGENT_ACCESS_GATEWAY_API_KEY` in the process environment and the file returned by `hermes config env-path`. If either value is missing, ask the user for both values in the conversation. Warn that an API key entered in chat remains in local Hermes session history. After the user responds, configure Hermes with the repository helper:

   ```bash
   AGENT_ACCESS_GATEWAY_API_KEY_INPUT="<user-api-key>" \
     python3 <source-directory>/scripts/configure-agent.py \
       --agent hermes \
       --gateway-url "<user-gateway-url>"
   ```

   Pass the API key only through the temporary process environment. Never include it in progress or final output. The helper updates the existing Hermes `.env` atomically, preserves unrelated settings, and sets file mode `0600`. Installed Gateway helpers read this `.env` directly when the current Hermes process has not loaded the new values yet.

   For another Agent, check whether its process already has both environment variables:

   - `AGENT_ACCESS_GATEWAY_URL`
   - `AGENT_ACCESS_GATEWAY_API_KEY`

   If either is missing, ask the user for it. Treat the API key as a secret. Do not print it, commit it, or write it into a project file. Configure it using the host's secret/environment mechanism.

7. Run `hermes skills list --source local` for Hermes and verify all four `gateway-*` skills appear. Tell the user installation and configuration succeeded. Ask them to begin a new Hermes chat so the Agent reloads the newly installed skill metadata. Restarting Hermes Gateway is optional because the helpers can read `.env` directly, but restart it before later background-channel use. Do not call live Gateway resources merely to test installation unless the user authorizes that resource use.

## Expected Result

The Agent can invoke:

- `gateway-google-account` for its assigned Google account.
- `gateway-google-auth` for a short-lived Google API token.
- `gateway-google-workspace` for Gmail and Drive operations.
- `gateway-remote-browser` for a persistent remote browser.

Each Gateway API key owns a separate Google account and Browser profile.
