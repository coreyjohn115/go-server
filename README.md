# due-server

基于 [due](https://github.com/dobyte/due) v2.6.7 的实时服务端脚手架：**gate 网关 + logic / battle 两个有状态节点 + backend 无状态 HTTP 后台服务**。

- 框架：`github.com/dobyte/due/v2 v2.6.7`（子模块固定在同一发布提交）
- Go：`go 1.27.0`（Go 1.21+ 会自动下载所需 toolchain）
- 传输：TCP（可换成 ws / kcp / quic）
- TCP 编解码：Protobuf（backend HTTP 仍为 JSON）
- 静态业务配置：Luban 5.1.0 + Excel，生成 Go 类型与 JSON 数据
- 玩家数据：MongoDB + 每个玩家一个 Actor

## 进程与职责

| 进程 | due 角色 | 状态 | 职责 |
| --- | --- | --- | --- |
| `cmd/gate` | Gate | 只维护连接 | TCP `:3553` 接入、按路由转发、按 UID 定位节点 |
| `cmd/logic` | Node（名称 `logic`） | **有状态** | 登录、玩家数据、经验/金币；数据存 MongoDB，玩家内部功能由 Actor 处理 |
| `cmd/battle` | Node（名称 `battle`） | **有状态** | 房间与战斗；进入战斗时把 UID 绑定到实例 |
| `cmd/backend` | HTTP 服务 | **无状态** | 登录校验、运营查询；不接入集群、不依赖 etcd/redis/mongo |
| `cmd/client` | Client | — | 调试客户端，演示完整链路 |

## 有状态路由与无状态路由

| 类型 | 行为 | 本项目示例 |
| --- | --- | --- |
| 无状态 | 网关按 `dispatch` 策略分发到任一实例 | 登录、ping、进入战斗 |
| 有状态 | 网关按「UID + 服务组」定位到已绑定实例 | 玩家数据、加经验、加金币、攻击、离开战斗 |

同一 UID 可以分别绑定到多个服务组：登录时绑定 `logic`，进入战斗时绑定 `battle`，两者互不影响。
有状态路由在 UID 未绑定时无法定位目标实例，所以客户端必须先走无状态入口（登录 / 进入战斗）。

## 玩家数据与 Actor

### 数据模型（MongoDB）

集合 `due.players`，以 UID 作为 `_id`：

```json
{
  "_id": 1001,
  "nickname": "player-1001",
  "level": 1,
  "exp": 50,
  "gold": 100,
  "version": 3,
  "createdAt": "2026-11-15T11:40:21Z",
  "updatedAt": "2026-11-15T11:40:21Z"
}
```

### 上线加载

登录流程（`internal/logic/handler`）：

1. backend 校验 token；
2. `BindGate` / `BindNode` 绑定 UID 到当前 logic 实例；
3. 创建或复用玩家 Actor：`playerstore.LoadOrCreate` 从 MongoDB 读取玩家，不存在则创建初始数据（level=1, exp=0, gold=0）；
4. `BindActor` 并在 Actor 执行域读取快照，返回玩家数据。登录失败的绑定回滚需结合会话归属机制完善。

### Actor 处理玩家内部功能

每个在线玩家对应一个 Actor，玩家内部系统功能都是 Actor 消息，串行执行：

| 路由 | 功能 | 谁处理 |
| --- | --- | --- |
| 110 | 查询玩家数据 | 玩家 Actor |
| 111 | 加经验（达到 `等级*100` 自动升级） | 玩家 Actor |
| 112 | 加金币 | 玩家 Actor |

节点级路由只做转交：

```go
func (h *Logic) NextToActor(ctx node.Context) {
    if err := ctx.Next(); err != nil { /* 回错误包 */ }
}
```

`ctx.Next()` 由 Scheduler 按「路由 → Actor 类型」「UID → Actor」定位到具体 Actor，之后
解析、改数据、回包都在 Actor 的串行执行域内完成，同一玩家的数据不需要加锁。

### 落库策略

- 数据变更先改 Actor 内存，标记 dirty；
- 首次变更后 500ms 调度一次 MongoDB 写入，后续变更不会重置定时器（`AfterInvoke` 在 Actor 执行域内串行执行，实际时间受队列与数据库延迟影响）；
- 临时写库失败保留 dirty 并在 500ms 后重试；保存使用上次成功写入的版本作条件，版本冲突后停止写入与资产变更，记录错误；
- 玩家下线或节点停服时，`Destroy` 尝试同步落库一次。强制终止、数据库持续故障或版本冲突仍可能丢失尚未保存的数据。
- 示例加经验/金币请求每次增量上限为 1,000,000，并检查数值溢出；正式资产应由服务端奖励/结算驱动，另行实现业务幂等。

## 路由表

| 路由号 | 名称 | 处理节点 | 类型 | 说明 |
| --- | --- | --- | --- | --- |
| 1 | login | logic | 无状态 | backend 校验 token，MongoDB 加载玩家，绑定 UID 并创建 Actor |
| 2 | ping | logic | 无状态 | 连通性测试 |
| 110 | playerData | logic | 有状态 + Actor | 查询玩家数据 |
| 111 | addExp | logic | 有状态 + Actor | 加经验，自动升级 |
| 112 | addGold | logic | 有状态 + Actor | 加金币 |
| 120 | itemList | logic | 有状态 + Actor | 背包列表 |
| 121 | useItem | logic | 有状态 + Actor | 使用道具 |
| 122 | deleteItem | logic | 有状态 + Actor | 删除道具 |
| 130 | taskList | logic | 有状态 + Actor | 任务列表 |
| 131 | commitTask | logic | 有状态 + Actor | 提交任务领取奖励 |
| 201 | enterBattle | battle | 无状态 + 需登录 | 绑定 UID 到 battle，创建房间 |
| 202 | attack | battle | 有状态 | 攻击，房间状态只在本实例 |
| 203 | leaveBattle | battle | 有状态 | 清理房间并解除 battle 绑定 |

路由号定义在 [internal/common/protocol/route.go](internal/common/protocol/route.go)，按业务域分段：`1-99` 通用、`100-199` logic、`200-299` battle。

## 链路

```text
client ──TCP:3553──▶ gate ──▶ logic（登录 / 玩家数据 / Actor）
                        └───▶ battle（进入 / 攻击 / 离开）

logic ──HTTP──▶ backend（登录令牌校验、运营查询）
logic ──MongoDB──▶ players 集合（加载 / 落库）

etcd：实例与路由发现        redis：UID -> 所在 gate / logic / battle
```

## backend HTTP 接口

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/healthz` | 健康检查 |
| GET | `/api/v1/server/info` | 服务信息（名称、due 版本、时间） |
| POST | `/api/v1/auth/verify` | 登录令牌校验，body：`{"uid":1001,"token":"dev-token"}` |
| GET | `/api/v1/players/:uid` | 玩家信息查询（运营后台示例） |

```bash
curl http://127.0.0.1:8080/healthz
curl -X POST http://127.0.0.1:8080/api/v1/auth/verify \
     -H 'Content-Type: application/json' \
     -d '{"uid":1001,"token":"dev-token"}'
```

## 目录结构

`internal` 按服务器类型划分目录，跨进程共用的模块统一放在 `internal/common`。

```text
due/
├── cmd/                          # 各进程入口（只做装配）
│   ├── gate/
│   ├── logic/
│   ├── battle/
│   ├── backend/
│   └── client/
├── internal/
│   ├── common/                   # 公共模块
│   │   ├── protocol/             # 路由号与消息结构（客户端共用）
│   │   ├── bootstrap/            # 配置中心 / 注册中心 / 定位器
│   │   └── backendapi/           # backend 接口契约 + HTTP 客户端
│   ├── gate/                     # 网关组件构建（网络、定位、注册）
│   ├── logic/                    # logic 节点
│   │   ├── app/                  # 路由/事件装配
│   │   ├── handler/              # 登录、转交 Actor、ping、连接事件
│   │   ├── actor/player/         # 玩家 Actor（内部系统功能、升级、合并落库）
│   │   └── store/playerstore/    # 玩家数据 MongoDB 读写
│   ├── battle/                   # battle 节点
│   │   ├── app/                  # 路由装配
│   │   └── handler/              # 进入、攻击、离开、房间状态
│   └── backend/                  # backend HTTP 服务
│       ├── app/                  # 路由装配
│       └── handler/              # HTTP 接口实现
├── etc/etc.toml                  # 框架启动配置
├── config/app.toml               # 业务配置（token、backend 地址、mongo）
├── docker-compose.yaml           # etcd + redis + mongo
├── doc/                          # 功能文档（新增功能必须同步补充）
│   ├── README.md                 # 文档索引与规范
│   └── features/                 # 一个功能一个文件（模板 _template.md）
├── .agents/skills/feature-docs/  # 项目 skill（跨 agent 通用目录）：功能文档规范
├── start.bat                     # Windows 一键启动服务器
└── README.md
```

## 功能文档

- 文档索引与规范：[doc/README.md](doc/README.md)
- [道具与背包](doc/features/items.md)、[玩家任务](doc/features/tasks.md)：参考 Game101 的初始化、道具事件和任务提交机制，通过 Luban 配置，在玩家 Actor 内处理并持久化。
- 新增或修改功能时，必须在 `doc/features/<feature>.md` 新增/更新功能文档，并更新索引；
- 具体规则与完成前检查清单见项目 skill [feature-docs](.agents/skills/feature-docs/SKILL.md)；
- 新功能从 `doc/features/_template.md` 复制开始。

## 快速开始

### Windows 根目录一键启动

在根目录运行 `start.bat`（需要 Go 和 Python 启动器 `py`），脚本先关闭本项目 `bin` 目录下的旧服务，等待退出后再编译，并分别打开 backend、logic、battle、gate 控制台。
关闭顺序为 gate、battle、logic、backend，通过 Ctrl+C 正常停服；每类服务最多等待 40 秒，无法关闭时中止启动。通过 `go run` 或其他目录启动的服务需手动关闭。
中间件需要提前启动；也可运行 `start.bat --compose`，同时启动 Docker Compose 中间件。
Docker MongoDB 映射到宿主机 `27018`，可与本机 `27017` 的 MongoDB 同时运行。
脚本输出表示进程已启动，各服务是否就绪请查看对应控制台；停止服务时在各控制台按 Ctrl+C，服务退出后窗口自动关闭。

### robot 登录测试

服务就绪后，在根目录运行：

```bat
go run ./cmd/robot
go run ./cmd/robot -addr 127.0.0.1:3553 -uid 2001 -count 10 -timeout 10s
```

`robot` 是独立测试客户端，不接入服务注册中心。每个机器人使用独立连接与递增 UID，
依次检查登录、玩家数据查询和 ping；全部完成后输出 PASS / FAIL 与汇总并退出。
默认从 UID 1001 开始，仅检查登录和读取链路，不增加经验或金币。
`-token` 可覆盖登录令牌，默认读取 `config/app.toml` 的 `[app].token`。
任一机器人失败时退出码为 1，参数无效时为 2。不要与其他在线客户端使用相同 UID。
超时包含拨号等待，拨号无法被 Ctrl+C 立即取消时最多等待配置的拨号超时。
`start.bat` 编译后也可直接运行 `bin\robot.exe`。

### 1. 启动中间件

```bash
docker compose up -d      # etcd :2379 / redis :6379 / mongo :27018
```

项目默认连接 Docker MongoDB `127.0.0.1:27018`，数据卷独立于 Windows 本机 MongoDB；切换连接不会自动迁移原有数据。修改连接配置后需重启 logic。

### 2. 启动四个服务（顺序不限，建议 backend 先起）

```bash
go run ./cmd/backend      # HTTP :8080
go run ./cmd/logic        # 需要 MongoDB，否则启动失败
go run ./cmd/battle
go run ./cmd/gate         # TCP :3553
```

### 3. 启动调试客户端

```bash
go run ./cmd/client
```

首次运行（新建玩家）：

```text
INFO login ok (from mongo): uid=1001 nickname=player-1001 level=1 exp=0 gold=0, logic node=...
INFO player data: uid=1001 nickname=player-1001 level=1 exp=0 gold=0, logic node=...
INFO add exp +50: uid=1001 nickname=player-1001 level=1 exp=50 gold=0, logic node=...
INFO add gold +100: uid=1001 nickname=player-1001 level=1 exp=50 gold=100, logic node=...
INFO player data: uid=1001 nickname=player-1001 level=1 exp=50 gold=100, logic node=...
INFO enter battle ok, room: room-1001, battle node: ...
INFO attack ok, room: room-1001, hits: 1/2/3, battle node: ...
INFO leave battle ok, room: room-1001, battle node: ...
INFO pong received: ...
```

重启 logic 后再运行客户端（数据从 MongoDB 恢复）：

```text
INFO login ok (from mongo): uid=1001 nickname=player-1001 level=1 exp=50 gold=100, logic node=...
INFO add exp +50: uid=1001 nickname=player-1001 level=2 exp=0 gold=100, logic node=...
INFO add gold +100: uid=1001 nickname=player-1001 level=2 exp=0 gold=200, logic node=...
```

## 扩展业务

**新增玩家内部功能（Actor）**

1. `internal/common/protocol/route.go` 加编号（`100-199` 段）与消息结构；
2. `internal/logic/actor/player/player.go` 加 `handleXxx`，在 `Init()` 中 `actor.AddRouteHandler`；
3. `internal/logic/app/logic.go` 注册节点级路由，指向 `h.NextToActor`（`node.RouteOptions{Stateful: true, Authorized: true}`）；
4. 客户端注册回包处理并发送请求。

**新增 battle 业务**

路由号放 `200-299` 段，写在 `internal/battle/handler`，在 [internal/battle/app/battle.go](internal/battle/app/battle.go) 注册（按需 Stateful）。

**新增 backend 接口**

路径常量加到 [internal/common/backendapi/backendapi.go](internal/common/backendapi/backendapi.go)，handler 写在 [internal/backend/handler](internal/backend/handler)，再在 [internal/backend/app/backend.go](internal/backend/app/backend.go) 注册。

**横向扩容**

- 多开 `go run ./cmd/logic`：登录、ping 会在多个实例间分摊；同一 UID 的玩家数据与 Actor 始终在其绑定实例。
- 多开 `go run ./cmd/battle`：新玩家按无状态策略进入不同实例；已进入战斗的玩家固定在原实例。
- backend 无状态，多实例前加负载均衡即可。

## 配置

| 文件 | 用途 |
| --- | --- |
| `etc/etc.toml` | 框架启动参数：网关、节点、网络、packet、日志、http 端口、注册中心、定位器 |
| `config/app.toml` | 业务配置，读取键：`app.app.token`、`app.backend.base_url`、`app.mongo.uri/database/collection` |

要点：

- `logic` / `battle` 共用 etc，节点名由 `node.WithName("logic")` / `node.WithName("battle")` 指定。
- gate、logic、battle、client 的 `packet` 参数必须完全一致；node/client 的 `codec` 使用 `proto`。

## 协议和配置生成

根目录执行 `gen.bat` 生成全部；`proto\gen.bat` 生成 Protobuf；`Excel\gen.bat` 使用 Luban 导表。
源文件在 `proto/message.proto`、`Excel/Data/*.xlsx` 和 `Excel/Defines/game.xml`。
生成代码及 `config/generated` 数据需随项目提交、部署。首次需要联网准备工具，后续复用本地缓存。
详细步骤见 [协议与配置生成](doc/features/code-generation.md)。此版本 TCP 协议不再兼容旧 JSON 客户端。
- 多进程共用 `etc.toml` 时不要配置 `pid`。

## 生产建议

- **数据一致性**：玩家数据以 Actor 为唯一写入口；`version` 条件更新阻止旧数据覆盖。有状态路由只保证请求按绑定转发，尚未实现跨实例登录互斥、旧会话隔离和 Actor 接管；多实例重复登录仍需完善归属协议。版本冲突后应排查重复 Actor，不应直接覆盖数据库。
- **落库**：当前为 500ms 合并写库 + 下线同步落库；对重要资产建议改为“事务 + 幂等 + 版本号条件更新”，并增加失败重试/补偿。
- **有状态节点扩缩容**：先摘流、保存状态、解绑或迁移用户，再缩容；`battle` 房间在内存中，重启会丢，需要持久化设计。
- **登录链路**：backend / MongoDB 故障会导致登录失败，建议加多实例、超时、降级与监控。
- **接入防护**：公网只暴露 gate；按需开启 `authorizeTimeout`、limiter、guard 与 TLS / 会话安全。
- **backend 安全**：示例只做固定 token 校验，生产应接入账号服务、鉴权与限流。

## 常见问题

**1. logic 启动失败：`connect mongo failed`**

MongoDB 没启动或地址不对。检查 `config/app.toml` 的 `[mongo].uri`（读取键 `app.mongo.uri`），默认 `mongodb://127.0.0.1:27018`。

**2. 玩家数据路由报错 / 没有响应**

该 UID 还未登录（没有绑定 Actor）。先调用登录路由；有状态路由与 Actor 路由都要求 UID 已绑定。

**3. 登录失败，logic 日志出现 `verify token via backend failed`**

backend 没启动或 `backend.base_url` 不对。

**4. Redis 版本**

due 定位器使用 `SET ... KEEPTTL`，需要 **Redis 6+**。

**5. 端口冲突**

默认占用 `3553`（gate）、`8080`（backend）、`2379`（etcd）、`6379`（redis）、`27018`（Docker MongoDB）；项目默认连接 Docker MongoDB `27018`。

## 已验证

- `go build ./...`、`go vet ./...`、`gofmt -l .`、`go test ./...` 通过（含升级/快照单测）。
- 本机 etcd v3.7.3 + Redis 8.10.3 + MongoDB 7.0 实测：
  - 首次登录创建文档，加经验 +50、加金币 +100，Actor 在 500ms 后合并落库；
  - 杀掉 logic 进程重启后再次登录，等级/经验/金币从 MongoDB 正确恢复（level=1, exp=50, gold=100），随后加经验触发升级到 level=2，金币变为 200 并再次落库。
