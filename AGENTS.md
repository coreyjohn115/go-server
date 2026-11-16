# AGENT.md

## Codex 工作目录

- 临时目录固定为 `.codex-work/<task>/`（脚本、检查结果、临时产物）；不要建 `outputs/` 等其它临时目录。
- **`node_modules` 只允许出现在 `.codex-work/node_modules`（公用）**。判断依赖 / `npm install` 前先检查此处；**禁止**在 `.codex-work/<task>/` 或其它子目录再装一份。
- 最终文件放入对应项目目录（如协议改动 → `internal/common/protocol/`，业务 handler → `internal/{logic,battle}/handler/`）。
- 任务结束：删临时文件；目录空了一并删掉，勿留空壳。

## 命令执行偏好

宿主终端可能是 PowerShell，但**禁止主动使用 PowerShell cmdlet / 语法**；只发 cmd 风格原生命令（`exe`、`go`、`docker`、`git`、`make`、`py` 等）。

- 搜索优先用编辑器/工具；命令行可用 `rg`（若已安装）或 `findstr`，勿探测 PATH 后降级成自写全盘扫描。
- 读文件：优先用编辑器/工具读取；命令行用 `type` 或 Python，**禁止** `Get-Content`。
- 文本处理 / 批量改名 / JSON·TOML 等用 Python（`py`）。
- **禁止** PowerShell 专有命令与语法，包括但不限于：`Get-Content`、`Get-ChildItem`、`Select-String`、`Where-Object`、`ForEach-Object`、`Set-Content`、`Out-File`、管道 `|` 接 cmdlet、here-string `@'...'@`、反引号续行、`$var` 变量展开拼复杂命令。除非系统/权限操作确无其它办法，否则不要用。
- `py -c` 仅极短单行；多行、复杂引号、正则、Windows 路径等必须写成 `.codex-work/<task>/tmp_xxx.py` 再 `py` 执行，用完删除。
- 不要用 PowerShell here-string / `\n` / 反引号续行拼复杂 Python。

## 项目概览

基于 [due](https://github.com/dobyte/due) v2 的实时服务端脚手架：**gate + logic / battle（有状态节点）+ backend（无状态 HTTP）**。

| 职责 | 路径 |
|------|------|
| 进程入口 | `cmd/{gate,logic,battle,backend,client}/` |
| 路由/事件装配 | `internal/{logic,battle,backend}/app/` |
| 业务 Handler | `internal/{logic,battle,backend}/handler/` |
| 网关组件 | `internal/gate/` |
| 玩家 Actor | `internal/logic/actor/player/` |
| 玩家持久化 | `internal/logic/store/playerstore/` |
| 协议（路由号 + 消息） | `internal/common/protocol/` |
| backend HTTP 契约/客户端 | `internal/common/backendapi/` |
| 注册中心/定位器/配置中心 | `internal/common/bootstrap/` |
| 框架启动配置 | `etc/etc.toml` |
| 业务配置（可热更新） | `config/app.toml` |
| 中间件编排 | `docker-compose.yaml` |
| 项目说明 | `README.md` |

## 常用命令

```bat
docker compose up -d
go run ./cmd/backend
go run ./cmd/logic
go run ./cmd/battle
go run ./cmd/gate
go run ./cmd/client
```

Windows 下可用根目录 `start.bat` 启动服务器，或用 `start.bat --compose` 同时启动 Docker 中间件。

修改后优先静态回归，不强制全量联调：

```bat
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

## 硬约束

- **沿用现有 due 分层**：`cmd` 只做装配；`app` 只注册路由/事件；业务写在 `handler`；协议写在 `common/protocol`；跨进程 HTTP 契约写在 `common/backendapi`。
- **有状态 / 无状态路由**：
  - 无状态：网关按 `dispatch` 分发到任一实例（登录、ping、进入战斗）。
  - 有状态：网关按「UID + 服务组」定位已绑定实例（玩家数据、攻击、离开战斗）。
  - 有状态路由前必须先走无状态入口完成绑定（登录绑定 `logic`，进入战斗绑定 `battle`）。
  - 注册有状态路由用 `node.RouteOptions{Stateful: true, Authorized: true}`。
- **路由号是协议契约**：定义在 `internal/common/protocol/route.go`，按域分段：`1-99` 通用、`100-199` logic、`200-299` battle。新增只追加，不复用、不改已有编号。
- **消息结构与路由同步**：改路由号或字段时，同步改 `internal/common/protocol/message.go`，并更新对应 handler / client。
- **logic 玩家数据**：同一玩家用一个 Actor（`kind=player`，`id=UID`）串行处理；数据变更先改内存再延迟落库；禁止在多个 goroutine 直接并发改同一玩家文档而不经 Actor。
- **阻塞 IO 放 Task**：登录等含 HTTP/DB 的逻辑用 `ctx.Task(...)`，回包写在任务内，外层 handler 立即返回。
- **battle 房间状态**：只保存在当前 battle 实例内存；进程重启会丢。缩容前需摘流/解绑；重要战斗需自行设计持久化。
- **配置分离**：框架参数只放 `etc/etc.toml`；业务参数放 `config/`，用 `config.Get` / `config.Watch`。多进程共用 etc 时不要配 `pid`。
- **编解码一致**：gate / logic / battle / client 的 `packet` 与 `codec` 必须一致（当前为 JSON + TCP）。
- **中间件版本**：定位器需要 **Redis 6+**（`SET ... KEEPTTL`）；etcd / redis 通过 `docker compose` 启动。backend 可不依赖中间件单独运行。
- **模块路径**：Go module 名为 `due-server`；新增包 import 前缀保持 `due-server/internal/...`。
- **不要回滚与任务无关的本地改动**。
- **不要提交密钥**：`.env`、生产 token、真实账号配置等勿入库；示例 token 仅限本地开发。

## 扩展业务速查

| 场景 | 步骤 |
|------|------|
| 新增 logic 业务 | `common/protocol` 加路由/消息 → `logic/handler` 或 `logic/actor/player` → `logic/app/logic.go` 注册 |
| 新增 battle 业务 | 路由号放 `200-299` → `battle/handler` → `battle/app/battle.go` 注册（按需 Stateful） |
| 新增 backend 接口 | `common/backendapi` 加路径常量 → `backend/handler` 实现 → `backend/app/backend.go` 注册 |
| 换注册中心/定位器 | 只改 `internal/common/bootstrap/` |

## 知识库索引

按任务先读对应入口，再动手改代码：

| 场景 | 入口 |
|------|------|
| 架构、链路、扩容、FAQ | `README.md` |
| 路由号与消息契约 | `internal/common/protocol/` |
| 登录 / Actor / 玩家数据 | `internal/logic/handler/`、`internal/logic/actor/player/`、`internal/logic/store/playerstore/` |
| 战斗房间 | `internal/battle/handler/` |
| backend HTTP | `internal/backend/handler/`、`internal/common/backendapi/` |
| 启动与中间件 | `start.bat`、`docker-compose.yaml`、`etc/etc.toml`、`config/app.toml` |
