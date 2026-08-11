# Agent Access Gateway

User-facing gateway for Browser and Google Workspace resources. Telegram and platform-level tokens are out of scope.

An administrator creates or rotates a user key through `POST /v1/admin/users`. Users authenticate every resource request with `Authorization: Bearer <gatewayApiKey>` or `X-Gateway-API-Key`.

## User routes

- `GET /v1/google-user` assigns once and repeatedly returns the same Google account and password.
- `GET /v1/google-user/access-token` issues a short-lived DWD token for that account.
- `GET /v1/browser` creates or returns the current Browser.
- `POST /v1/browser/reset` replaces the current Browser session.
- `POST /v1/browser/close` stops the current session while retaining its profile for the next start.

## Run

Configure PostgreSQL, Browser Use Cloud, and Google Workspace in `cmd/accessgateway/config.yaml`, then run:

```bash
go run ./cmd/accessgateway --config ./cmd/accessgateway/config.yaml
```

Google uses separate service accounts: `google.creation` creates Workspace users through Admin SDK, while `google.authorization` issues delegated Gmail/Drive tokens. Do not share one credentials file between these roles.

Google passwords are intentionally stored in plaintext to support repeated account retrieval. Protect the database and its backups as secrets.
