# Hymatrix Module 生成教程

本文说明如何从 Hub 仓库中的源码生成可供 Hymatrix/VMDocker 使用的 Module。

## 1. 目录与产物

相关目录如下：

```text
hymatrix-module/
├── start-hermes/          # Go 启动程序、四个 Skills、browser-harness
├── module/
│   ├── profile.toml       # Module 构建配置
│   └── bin/start-hermes   # build.sh 生成，不提交 Git
└── scripts/build.sh       # 编译 start-hermes
```

生成过程分为两步：

1. 将 `start-hermes` 编译成与基础镜像架构一致的 Linux 二进制。
2. 使用 `hype vmdocker module build` 将基础镜像、二进制和 Profile 打包并发布为 Module。

## 2. 准备环境

需要安装：

- Docker，并确保 Docker daemon 已启动；
- Go 1.25 或兼容版本；
- `hype` CLI；
- 已编译的 `vmdocker-agent`；
- 用于签署 Module 交易的私钥。

本教程假设：

```text
Hub 仓库：         /path/to/hub
VMDocker v2：      /path/to/vmdocker-workspace/vmdockerv2
vmdocker-agent：   /path/to/vmdocker-agent
```

请将示例中的路径换成实际绝对路径。

## 3. 检查 Profile

构建配置位于 `hymatrix-module/module/profile.toml`：

```toml
[dockerfile]
FROM = "sandytest456/hermes-agent:linux-full"
bin = "bin"
CMD = ["start-hermes"]
```

- `FROM` 是 Module 使用的 Hermes 基础镜像；
- `bin` 表示把 `module/bin/` 放入 Module；
- `CMD` 让容器创建后自动运行 `start-hermes`。

如果更换基础镜像，需要确保镜像包含 Hermes、Python 和 `uv`。

## 4. 编译 start-hermes

进入 Hub 仓库根目录：

```sh
cd /path/to/hub
```

让脚本自动读取基础镜像架构并编译：

```sh
./hymatrix-module/scripts/build.sh
```

也可以显式指定架构：

```sh
./hymatrix-module/scripts/build.sh arm64
./hymatrix-module/scripts/build.sh amd64
```

脚本会依次执行：

- `go test ./...`；
- `go vet ./...`；
- Linux 静态交叉编译；
- `go version -m` 校验产物；
- 将最终文件写入 `hymatrix-module/module/bin/start-hermes`。

验证二进制：

```sh
file ./hymatrix-module/module/bin/start-hermes
```

输出应为 Linux ELF，并且架构与基础镜像一致。例如：

```text
ELF 64-bit LSB executable, ARM aarch64, statically linked
```

## 5. 准备签名私钥

通过环境变量传入私钥，避免直接写进命令历史或仓库文件：

```sh
export VMDOCKER_PRIVATE_KEY="<your-private-key>"
```

不要提交私钥、Gateway API Key、LLM API Key 或 Telegram Bot Token。

## 6. 生成 Module

执行：

```sh
hype vmdocker module build \
  --dir /path/to/vmdocker-workspace/vmdockerv2 \
  --profile /path/to/hub/hymatrix-module/module/profile.toml \
  --agent-bin /path/to/vmdocker-agent \
  --private-key "$VMDOCKER_PRIVATE_KEY"
```

参数含义：

- `--dir`：VMDocker v2 源码目录；
- `--profile`：本项目的 Module Profile；
- `--agent-bin`：与目标运行环境匹配的 `vmdocker-agent`；
- `--private-key`：签署和发布 Module 的私钥。

命令成功后会输出 Module ID。把该 ID 填入 Hub 管理后台 Hymatrix 页面中的
`Module` 字段。

## 7. Spawn 时注入运行配置

Module 本身不保存用户密钥。Hub Manager 会在 Spawn 交易中使用
`Container-Env-*` Tags 注入：

```text
RUNTIME_TYPE
HERMES_AGENT_LLM_PROVIDER
HERMES_AGENT_LLM_MODEL
HERMES_AGENT_LLM_BASE_URL
HERMES_AGENT_LLM_API_KEY
HUB_GATEWAY_URL
HUB_GATEWAY_API_KEY
HERMES_AGENT_TELEGRAM_BOT_TOKEN（可选）
```

容器启动后，`start-hermes` 会：

1. 安装内嵌的四个 Gateway Skills；
2. 安装并验证内嵌的 `browser-harness`；
3. 写入 Hermes 环境和 LLM 配置；
4. 前台执行 `hermes gateway run`。

无需再发送单独的 Start 交易。

> 安全提示：`Container-Env-*` Tags 不加密，密钥会对交易处理节点可见。仅在可信 Hymatrix 网络中使用。

## 8. 修改 Skills 后重新生成

Skills 的唯一源码目录是：

```text
hymatrix-module/start-hermes/skills/
```

修改 Skill 后必须重新执行：

```sh
./hymatrix-module/scripts/build.sh
hype vmdocker module build ...
```

Skills 通过 Go `embed` 编译进 `start-hermes`，只修改源码但不重新编译不会更新已有 Module。

## 9. 常见问题

### Docker 镜像架构无法识别

先拉取基础镜像：

```sh
docker pull sandytest456/hermes-agent:linux-full
```

或者显式向构建脚本传入 `arm64` 或 `amd64`。

### Module 启动后提示找不到 Hermes

确认 `profile.toml` 的基础镜像包含 `hermes` 命令，且命令位于 `PATH`、
`~/.local/bin/hermes`、`~/.hermes/hermes-agent/venv/bin/hermes` 或
`/usr/local/bin/hermes`。

### start-hermes 提示缺少 Gateway 配置

确认 Spawn Tags 中同时存在：

```text
Container-Env-HUB_GATEWAY_URL
Container-Env-HUB_GATEWAY_API_KEY
```

这两个变量必须成对提供，不能只设置其中一个。

### 修改代码后 Module 行为没有变化

确认已重新运行 `build.sh`，并使用新的构建产物生成了新 Module。旧 Module ID
仍指向旧版本，不会自动更新。
