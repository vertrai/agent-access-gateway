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
├── tools/
│   ├── vmdocker-agent     # 本地平台 Adapter，Git 忽略
│   └── vmdockerv2/        # hype 自动准备的构建引擎，Git 忽略
└── scripts/
    ├── build.sh           # 编译 start-hermes
    └── build-module.sh    # 生成 Module
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
vmdocker-agent：   /path/to/vmdocker-agent
```

VMDocker v2 不需要指向其他项目。首次生成 Module 时，脚本会自动准备在：

```text
/path/to/hub/hymatrix-module/tools/vmdockerv2
```

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

## 5. 准备 vmdocker-agent

`vmdocker-agent` 是平台 Adapter，VMDocker 构建时会把它注入镜像的
`/usr/local/bin/vmdocker-agent`。不要把它放进 `module/bin/`；该目录只存放
`start-hermes` 等业务程序。

本项目约定的本地位置是：

```text
hymatrix-module/tools/vmdocker-agent
```

当前开发机可执行：

```sh
cp /Users/sandyzhou/GolandProjects/vmdockerv2_agent/build/vmdocker-agent \
  ./hymatrix-module/tools/vmdocker-agent
chmod 0755 ./hymatrix-module/tools/vmdocker-agent
```

该文件已被 `.gitignore` 忽略，因为它体积较大、与目标架构相关，并且可从
`vmdockerv2_agent` 重新构建。

检查其架构：

```sh
file ./hymatrix-module/tools/vmdocker-agent
```

它必须是 Linux 可执行文件，并且与 `profile.toml` 基础镜像架构一致。

## 6. Module 签名私钥

通常不需要手动准备。`build-module.sh` 在没有设置
`VMDOCKER_PRIVATE_KEY` 时，会通过 `openssl rand` 从系统加密随机源生成一个
32 字节的一次性私钥。它仅保留在脚本进程内，不会打印或写入文件。

直接运行即可：

```sh
./hymatrix-module/scripts/build-module.sh
```

如果构建和后续管理必须使用固定签名身份，也可以显式传入私钥：

```sh
export VMDOCKER_PRIVATE_KEY="<your-private-key>"
```

显式提供的私钥优先于自动生成值。不要提交私钥、Gateway API Key、LLM API
Key 或 Telegram Bot Token。

## 7. 生成 Module

推荐使用包装脚本；默认会自动生成一次性签名私钥：

```sh
./hymatrix-module/scripts/build-module.sh
```

脚本默认使用：

```text
VMDocker 工作区：hymatrix-module/tools/vmdockerv2
Agent 二进制：   hymatrix-module/tools/vmdocker-agent
Profile：        hymatrix-module/module/profile.toml
```

如果 VMDocker 工作区不存在，脚本会先执行：

```sh
hype vmdocker get --dir ./hymatrix-module/tools/vmdockerv2
```

`get` 只负责准备构建所需的 checkout。`hype vmdocker init` 会进一步启动本地
VMDocker 服务并运行 examples 初始化，单纯构建 Module 不需要执行它。

如需复用已有 checkout，也可以覆盖路径：

```sh
VMDOCKER_WORKSPACE_DIR=/path/to/vmdockerv2 \
VMDOCKER_AGENT_BIN=/path/to/vmdocker-agent \
./hymatrix-module/scripts/build-module.sh
```

等价的完整命令是：

执行：

```sh
hype vmdocker module build \
  --dir /path/to/hub/hymatrix-module/tools/vmdockerv2 \
  --profile /path/to/hub/hymatrix-module/module/profile.toml \
  --agent-bin /path/to/vmdocker-agent \
  --private-key "$VMDOCKER_PRIVATE_KEY"
```

参数含义：

- `--dir`：VMDocker v2 源码目录；
- `--profile`：本项目的 Module Profile；
- `--agent-bin`：与目标运行环境匹配的 `vmdocker-agent`；
- `--private-key`：签署和发布 Module 的私钥。

`--dir` 指向 `vmdockerv2` checkout，是因为当前 `hype vmdocker module build`
把它作为 Module 构建引擎：由其中的代码读取 Profile、准备 Docker 构建上下文、
把 `vmdocker-agent` 注入为镜像 ENTRYPOINT，并构建和发布 Module。它不会被打包
进最终 Module，也不是容器运行时依赖。

命令成功后会输出 Module ID。把该 ID 填入 Hub 管理后台 Hymatrix 页面中的
`Module` 字段。

## 8. Spawn 时注入运行配置

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

## 9. 修改 Skills 后重新生成

Skills 的唯一源码目录是：

```text
hymatrix-module/start-hermes/skills/
```

修改 Skill 后必须重新执行：

```sh
./hymatrix-module/scripts/build.sh
./hymatrix-module/scripts/build-module.sh
```

Skills 通过 Go `embed` 编译进 `start-hermes`，只修改源码但不重新编译不会更新已有 Module。

## 10. 常见问题

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
