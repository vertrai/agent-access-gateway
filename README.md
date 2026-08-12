# Agent Access Gateway

为 Codex、Claude、Hermes 等 Agent 提供可直接使用的 Google 账号、Gmail、Google Drive、远程浏览器和 Telegram Bot。

安装 Skills 后，你可以直接对 Agent 说：

- “申请一个 Google 账号”
- “查看邮箱里的未读邮件”
- “发送一封邮件给 user@example.com”
- “把这个文件上传到 Google Drive 并返回分享链接”
- “使用远程浏览器打开 example.com”

## 安装 Skills

安装前，请向 Agent Access Gateway 管理员获取：

- Gateway URL
- Gateway API Key（以 `gw_sk_` 开头）

然后把下面这段话完整发送给你的 Agent：

```text
请按照这个安装文档安装 Agent Access Gateway Skills：
https://raw.githubusercontent.com/vertrai/agent-access-gateway/main/agent-skills/INSTALL.md

需要配置时向我询问 Gateway URL 和 API Key。不要在输出中显示 API Key。
```

Agent 会自动完成下载、安装和配置。它询问时，再分别提供 Gateway URL 和 API Key。

### Hermes 用户

安装完成后，在当前 Hermes 对话中发送：

```text
/reload-skills
```

不需要重启 Hermes 服务。热加载完成后即可直接使用。

## 可以使用的能力

### Google 账号

```text
申请一个 Google 账号
获取我的 Google user
告诉我 Google 邮箱地址
使用 Google 账号登录网页
```

同一个 Gateway API Key 会一直使用同一个 Google 账号。

### Gmail

```text
查看 Google 邮箱
列出最近 7 天的未读邮件
读取这封邮件
发送一封测试邮件
创建一封邮件草稿
```

### Google Drive

```text
列出 Drive 中的文件
创建一个文件夹
创建一个文本文档
上传这个文件并返回分享链接
下载这个 Drive 文件
```

新创建或上传的 Drive 内容默认设置为“知道链接的人可以查看”，不会默认授予编辑权限。

### 远程浏览器

```text
使用远程浏览器打开 example.com
在网页中点击登录按钮
填写这个表单
返回浏览器实时查看链接
重置远程浏览器
关闭浏览器会话
```

同一个 Gateway API Key 会复用同一个浏览器 Profile，以保留网站登录状态。

## 常见问题

### Agent 没有调用 Gateway Skills

Hermes 用户先发送：

```text
/reload-skills
```

然后重新描述任务，例如“查看 Google 邮箱”或“使用远程浏览器打开网站”。

### 获取 Google 账号失败

账号池为空时，Gateway 会自动创建一个 Workspace 账号并立即分配。首次请求可能因此耗时更长。如果自动创建失败，检查服务端返回的 Google Workspace 错误和创建凭据；不要自行注册或切换其他 Google 账号方案。

### Gateway URL 无法连接

确认 Agent 所在环境能够访问 Gateway URL。在 Docker 中访问宿主机服务时，通常使用：

```text
http://host.docker.internal:8085
```

### 如何更新 Skills

再次把上面的安装文档链接发送给 Agent，让它重新执行安装流程即可。

## 服务端开发

以下内容仅供部署 Agent Access Gateway 服务的管理员和开发者使用；普通 Skills 用户不需要执行。

复制示例配置，填写 PostgreSQL、Browser Use 和 Google Workspace 配置：

```bash
cp ./cmd/resouces/config.example.yaml ./cmd/resouces/config.yaml
go run ./cmd/resouces --config ./cmd/resouces/config.yaml
```

后台管理页面：

```text
http://<gateway-host>:8085/admin
```

API 测试页面：

```text
http://<gateway-host>:8085/admin/test
```

请勿提交 `config.yaml`、Google Service Account JSON、Gateway API Key 或其他 credentials。Google 账号创建和 Access Token 授权必须使用两个独立的 Service Account。

### 服务目录

- `cmd/resouces` + `resouces`：资源网关服务。Browser、Google、Telegram 的供应商实现分别位于各自子目录。
- `cmd/manager` + `manager`：资源申请管理服务骨架，当前仅提供 `/info` 健康接口，尚未实现业务。
- `web`：`resouces` 和 `manager` 共用的后台管理前端，两套后端均挂载 `/admin` 与 `/admin/test`。
- `agent-skills`：Skills、安装脚本和 Agent 可执行安装文档的统一目录。

### Telegram Bot 资源

Telegram Bot 支持两种入池方式：管理员手动导入已有 Bot，或者授权 Telegram 用户账号后让服务通过 BotFather 自动创建。

手动导入已有 Bot：

```http
POST /v1/admin/telegram/bots
X-Admin-API-Key: <admin-key>
Content-Type: application/json

{"botToken":"<telegram-bot-token>","username":"example_bot"}
```

Agent 使用 Gateway API Key 获取固定分配给该 Key 的 Bot；账号池为空时返回 `503`：

```http
GET /v1/telegram-bot
Authorization: Bearer <gateway-api-key>
```

同一个 Gateway API Key 最多绑定一个 Telegram Bot，重复获取会返回同一个 Token。管理员列表接口 `GET /v1/admin/telegram/bots` 只返回脱敏 Token。

自动创建流程需要管理员依次调用：

```text
POST /v1/admin/telegram/auth/init       # phone、apiId、apiHash，发送验证码
POST /v1/admin/telegram/auth/verify     # accountId、code
POST /v1/admin/telegram/auth/2fa        # 仅账号启用 2FA 时调用
GET  /v1/admin/telegram/auth/status
GET  /v1/admin/telegram/auth/accounts
POST /v1/admin/telegram/bots/create     # body: {"count": 1}
```

授权完成后，服务每分钟检查一次可用 Bot 数量，并通过 BotFather 自动补充到 `telegram.minAvailableBots`。遇到 Telegram `FLOOD_WAIT` 会进入冷却期，期间自动创建接口返回 `429`，而手动导入仍然可用。Telegram MTProto Session 保存在 `telegram.dataDir`，必须使用持久化私有目录，不能提交到 Git。
