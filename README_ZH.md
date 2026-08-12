# Agent Access Gateway

Agent Access Gateway 直接面向终端用户提供 Browser 和 Google Workspace 资源，不包含 Telegram，也不再使用 Platform Token 和资源二级 Key。

## 鉴权

后台管理页面位于 `/admin`，可以手动创建用户、为用户新增 Gateway API Key，并复制后分发给用户。当前没有用户注册、登录或自助申请系统。

API 测试页面位于 `/admin/test`，可以使用已分发的 Gateway API Key 测试 Google User、Google Access Token、Browser 获取和 Browser 重置接口。
测试页也支持使用该 Google User 的 DWD Token 发送真实测试邮件，以及在 My Drive 根目录创建真实测试文件夹。

管理员调用 `POST /v1/admin/users` 创建用户或为已有用户新增 Key。一个用户可以拥有多个 Key，每个 Key 独立绑定一个 Browser 和一个 Google User。明文 Gateway API Key 只在这次响应中返回，PostgreSQL 只保存 Hash。

用户后续统一使用：

```http
Authorization: Bearer <gatewayApiKey>
```

也支持：

```http
X-Gateway-API-Key: <gatewayApiKey>
```

## 用户接口

- `GET /v1/google-user`：首次调用从账号池分配 Google User，之后始终返回同一账号和密码。
- `GET /v1/google-user/access-token`：为该 Google User 签发短期 DWD Access Token。
- `GET /v1/browser`：创建或返回当前 Browser。
- `POST /v1/browser/reset`：停止并重新创建 Browser。
- `POST /v1/browser/close`：停止当前 Browser Session，保留 profile 供下次启动复用。

## 管理接口

- `POST /v1/admin/users`：创建 Gateway User 或为用户新增 API Key。
- `POST /v1/admin/google/accounts`：使用 Admin SDK 创建 Google Workspace 用户并加入账号池。
- `POST /v1/admin/google/accounts/batch`：一次批量生产 1–100 个 Google Workspace 用户并加入账号池。
- `GET /v1/admin/google/accounts`：查看账号池和分配状态。

Google 用户密码按业务要求明文保存，以允许用户重复读取账号和密码。数据库和备份必须作为敏感数据保护。

## 启动

创建 PostgreSQL 数据库，复制示例配置并填写本地凭据：

```bash
cp ./cmd/accessgateway/config.example.yaml ./cmd/accessgateway/config.yaml
```

`config.yaml` 和 `cmd/accessgateway/*.json` 已被 Git 忽略，不要提交真实 API Key 或 Google Service Account JSON。然后运行：

```bash
go run ./cmd/accessgateway --config ./cmd/accessgateway/config.yaml
```

Google 使用两个独立的 Service Account：`google.creation` 只负责通过 Admin SDK 创建 Workspace 用户；`google.authorization` 只负责模拟已分配用户并签发 Gmail/Drive Access Token。两者不要共用 credentials 文件。

Google Access Token 按用户邮箱缓存在 Gateway 进程内。剩余有效期超过 5 分钟时直接复用；不足 5 分钟时自动刷新。同一用户的并发刷新会合并为一次 Google token 请求，Token 不写入 PostgreSQL。

Browser 按 Gateway API Key 一对一缓存。默认 30 秒内直接复用 PostgreSQL 中的 Session 连接信息，超过 30 秒调用 Browser Use 查询真实状态；服务方已停止、删除或 Session 临近超时时，Gateway 使用该 API Key 原有的 Profile ID 创建新 Session，以继续复用 Profile 中保存的登录态。同一进程内同一 API Key 的并发创建会被串行化。

## Agent Skills

项目的 `skills/` 目录提供四个可独立安装的通用 Skill：

- `gateway-google-account`：取得 API Key 对应的 Google 账号和密码。
- `gateway-google-auth`：取得该账号的 Google Access Token。
- `gateway-google-workspace`：直接操作 Gmail 和 Google Drive。
- `gateway-remote-browser`：取得、重置和关闭远程 Browser Session。

安装到支持 `SKILL.md` 的 Agent 时，将它们复制到该 Agent 的 skills 目录。例如：

```bash
./scripts/install-skills.sh ~/.codex/skills
./scripts/install-skills.sh ~/.claude/skills
./scripts/install-skills.sh ~/.hermes/skills
```

Agent 进程需要配置：

```bash
export AGENT_ACCESS_GATEWAY_URL="https://gateway.example.com"
export AGENT_ACCESS_GATEWAY_API_KEY="gw_sk_..."
```

每个 Agent 或隔离身份应使用独立的 Gateway API Key，使其拥有独立的 Google User 和 Browser profile。Google Skill 脚本仅依赖 Python 3 标准库；Remote Browser Skill 明确依赖 `browser-harness`，安装脚本会通过 `uv tool install browser-harness` 自动安装。连接脚本负责 attach daemon，并在连接失效时 reset 一次后重连。

也可以把公开的 [`INSTALL-SKILLS.md`](./INSTALL-SKILLS.md) 链接直接发送给 Agent，并要求它完整执行文档。Agent 会识别平台、克隆本仓库、安装并验证四个 Skill；缺少 Gateway URL 或 API Key 时才会向用户询问。

在 Hermes 对话式安装中，Hermes 会询问 Gateway URL 和 API Key，然后使用 `scripts/configure-agent.py` 原子更新 `hermes config env-path` 返回的 `.env`，文件权限保持为 `0600`。Helper 在当前 Hermes 进程未重载环境变量时也会直接读取该文件。安装后新建一次 Hermes chat，使新会话加载四个 Skill 的触发信息。
