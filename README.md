# Agent Access Gateway

为 Codex、Claude、Hermes 等 Agent 提供可直接使用的 Google 账号、Gmail、Google Drive 和远程浏览器。

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
https://raw.githubusercontent.com/vertrai/agent-access-gateway/main/INSTALL-SKILLS.md

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

### 获取 Google 账号返回 503

通常表示服务端账号池暂时为空。联系 Gateway 管理员补充账号后，再次执行相同请求即可。不要自行注册或切换其他 Google 账号方案。

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
cp ./cmd/accessgateway/config.example.yaml ./cmd/accessgateway/config.yaml
go run ./cmd/accessgateway --config ./cmd/accessgateway/config.yaml
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
