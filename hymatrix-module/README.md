# Hub Gateway Hermes Module

完整的中文生成步骤请参阅 [Hymatrix Module 生成教程](./BUILD_MODULE_ZH.md)。

## 最短构建流程

准备好与线上节点相同架构的 Linux `vmdocker-agent` 后，在 Hub 根目录运行：

```sh
chmod 0755 ./hymatrix-module/tools/vmdocker-agent
./hymatrix-module/scripts/build-module.sh amd64
```

可上传产物位于 `hymatrix-module/build/mod-<MODULE_ID>.json`；本地测试镜像为
`vmdocker-module:latest`。构建脚本会自动生成一次性签名私钥、编译
`start-hermes`、校验所有二进制与镜像架构，并归档 Module JSON。

## 简要运行链路

```text
Spawn 交易
  → Hymatrix 节点创建容器并注入 Container-Env-* Tags
  → vmdocker-agent 启动 Module CMD
  → start-hermes 安装 Skills、写入配置并启动 Hermes Gateway
  → /vmm/health 检查 Hermes HTTP API 的 127.0.0.1:8642/health
```

`vmdocker-agent` 是容器入口和 VMDocker runtime 适配器；`start-hermes` 是
`profile.toml` 指定的业务 CMD；Hermes Gateway 才是真正处理 Telegram、Skills
和 HTTP API 的进程。Module 会随 Spawn 自动启动，不需要额外发送 Start 交易。

本机可把预编译的 `vmdocker-agent` 放在 `tools/vmdocker-agent`。该平台二进制被
Git 忽略，不会提交或直接放入 `module/bin/`；`scripts/build-module.sh` 会默认
使用它。

Module 构建所需的 VMDocker checkout 默认位于 `tools/vmdockerv2/`，同样被 Git
忽略。首次构建时脚本会自动通过 `hype vmdocker get` 准备，不依赖其他项目目录。

未设置 `VMDOCKER_PRIVATE_KEY` 时，构建脚本会从系统加密随机源生成仅供本次
构建使用的一次性签名私钥，不打印也不保存。

该目录参考 `hermes-x-module` 的启动方式，将 Hub 的四个 Gateway Skills 嵌入
`start-hermes`。Module 每次启动时会把 Skills 刷新到
`~/.hermes/skills/`，写入 Hub Gateway 环境变量，然后以前台方式执行
`hermes gateway run`。

`gateway-remote-browser` 依赖的 `browser-harness` 源码也嵌入二进制，启动时通过
镜像内的 `uv` 从本地源码安装并验证，无需在线下载该包。

内置 Skills：

- `gateway-google-account`
- `gateway-google-auth`
- `gateway-google-workspace`
- `gateway-remote-browser`

`start-hermes/skills/` 是这四个 Skills 在仓库中的唯一源码位置。对外的
`agent-skills/scripts/install-skills.sh` 也从该目录安装，不要在
`agent-skills/` 下维护副本。

## 构建启动二进制

```sh
./hymatrix-module/scripts/build.sh
```

也可以显式指定基础镜像架构：

```sh
./hymatrix-module/scripts/build.sh arm64
./hymatrix-module/scripts/build.sh amd64
```

## 构建 Hymatrix/VMDocker Module

```sh
hype vmdocker module build \
  --dir /path/to/hub/hymatrix-module/tools/vmdockerv2 \
  --profile /path/to/hub/hymatrix-module/module/profile.toml \
  --agent-bin /path/to/vmdocker-agent \
  --private-key "$VMDOCKER_PRIVATE_KEY"
```

推荐明确指定线上节点架构，例如运行
`./hymatrix-module/scripts/build-module.sh amd64`。脚本会编译并校验同架构的
`start-hermes`，校验 `vmdocker-agent`，并强制 Docker 使用目标平台。最终生成的
`mod-<MODULE_ID>.json` 归档到 `hymatrix-module/build/`；可通过
`VMDOCKER_BUILD_DIR` 覆盖输出目录。
同时会把本次哈希镜像标记为 `vmdocker-module:latest`，供固定的本地测试命令
使用。

Module Profile 会在构建阶段预装并验证 Telegram adapter 和 Hermes HTTP API
Server 依赖，确保新容器首次启动即可加载 Telegram 并监听 8642，而不依赖运行时
动态安装。配置 Telegram 时，`start-hermes` 还会安装自动 Home Channel 插件，
首次私聊会直接成为 cron 和跨平台消息的投递会话。

## 运行环境变量

必须提供完整的新变量对：

```text
HUB_GATEWAY_URL
HUB_GATEWAY_API_KEY
```

为兼容已有 Pod，也支持完整的旧变量对：

```text
AGENT_ACCESS_GATEWAY_URL
AGENT_ACCESS_GATEWAY_API_KEY
```

当两套变量都完整时优先使用 `HUB_GATEWAY_*`，不会混用两套变量。LLM、
Telegram、API Server 等 Hermes 配置继续由 Module Spawn 参数或基础镜像负责。

Hermes HTTP API 和 VMDocker readiness 还需要：

```text
API_SERVER_ENABLED=true
API_SERVER_KEY=<random-secret>
HERMES_GATEWAY_TOKEN=<same-random-secret>
```
