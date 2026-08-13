# Hub Gateway Hermes Module

完整的中文生成步骤请参阅 [Hymatrix Module 生成教程](./BUILD_MODULE_ZH.md)。

本机可把预编译的 `vmdocker-agent` 放在 `tools/vmdocker-agent`。该平台二进制被
Git 忽略，不会提交或直接放入 `module/bin/`；`scripts/build-module.sh` 会默认
使用它。

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
  --dir /path/to/vmdocker-workspace/vmdockerv2 \
  --profile /path/to/hub/hymatrix-module/module/profile.toml \
  --agent-bin /path/to/vmdocker-agent \
  --private-key "$VMDOCKER_PRIVATE_KEY"
```

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
