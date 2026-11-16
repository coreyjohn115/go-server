# 玩家数据（MongoDB + Actor）

> 状态：已实现
> 涉及进程：logic（存储与 Actor）、backend（登录校验）、gate / client（链路）
> 更新日期：2026-11-16

## 1. 功能概述

玩家登录时从 MongoDB 加载数据（不存在则创建），在线期间的数据变更由每个玩家独立的
Actor 串行处理，变更合并后落库；玩家下线或 Actor 销毁时同步落库。

当前字段：昵称、等级、经验、金币，以及[背包](items.md)和[任务](tasks.md)。登录/查询/资产变更快照包含完整 `items`、`tasks`。

## 2. 路由与接口

| 路由号 | 类型 | 处理进程 | 说明 |
| --- | --- | --- | --- |
| `1` | 无状态 | logic | 登录：backend 校验 token、MongoDB 加载、绑定 UID、创建 Actor |
| `110` | 有状态 + Actor | logic | 查询玩家数据 |
| `111` | 有状态 + Actor | logic | 增加经验，按 Luban 等级表升级 |
| `112` | 有状态 + Actor | logic | 增加金币 |

## 3. 协议示例

TCP 采用 Protobuf，定义在 `proto/message.proto`。以下 JSON 仅用于说明字段，实际传输为二进制。
增加经验：

```json
{ "delta": 50 }
```

响应：

```json
{
  "code": 0,
  "message": "add exp ok",
  "player": {
    "uid": 1001,
    "nickname": "player-1001",
    "level": 2,
    "exp": 0,
    "gold": 100
  },
  "node": "01a1df02-916e-78ba-ba5d-0ae4bc83afe7"
}
```

错误码：

| code | 含义 | 触发条件 |
| --- | --- | --- |
| 3 | 参数错误 | `delta <= 0` 或消息解析失败 |
| 7 | 未授权 | 登录 token 校验失败 |
| 5 / 6 | 未找到 / 内部错误 | Actor 未绑定、落库失败等 |

## 4. 数据与存储

MongoDB 集合 `due.players`，以 UID 作为 `_id`：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `_id` | int64 | 玩家 UID |
| `nickname` | string | 昵称 |
| `level` | int32 | 等级，初始 1 |
| `exp` | int64 | 当前经验 |
| `gold` | int64 | 金币 |
| `items` | array | 堆栈 id/configId/count |
| `tasks` | array | 任务 id/progress/status，已提交记录保留 |
| `initItemIds` | array | 已发放初始道具的配置 ID |
| `nextItemId` | int64 | 玩家内最近分配的堆栈 ID |
| `version` | int64 | 每次变更自增，保存时检查上次成功写入版本 |
| `createdAt` | datetime | 创建时间 |
| `updatedAt` | datetime | 最近更新时间 |

- 读取：登录时 `LoadOrCreate`，不存在则插入初始值；
- 写入：`Save` 按 `_id + expectedVersion` 条件 `$set` 更新，版本冲突停止写入与资产变更；
- 内存态：数据只存在于该玩家 Actor 中，进程重启后从 MongoDB 恢复。

## 5. 配置项

| 配置 | 位置 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `app.mongo.uri` | `config/app.toml` | `mongodb://127.0.0.1:27018` | MongoDB 地址 |
| `app.mongo.database` | `config/app.toml` | `due` | 数据库名 |
| `app.mongo.collection` | `config/app.toml` | `players` | 集合名 |
| `app.app.token` | `config/app.toml` | `dev-token` | 示例登录令牌 |
| `app.backend.base_url` | `config/app.toml` | `http://127.0.0.1:8080` | 登录校验服务 |
| `app.tables.dir` | `config/app.toml` | `config/generated` | Luban 表数据目录，重启生效 |

## 6. 关键实现

- 路由注册：`internal/logic/app/logic.go`；节点级 handler 只调用 `ctx.Next()` 转交 Actor；
- Actor：`kind=player`、`id=UID`，见 `internal/logic/actor/player/`，`Init` 中注册 110/111/112；
- 登录：`ctx.Task` 中完成 HTTP 校验、MongoDB 加载、`BindGate`/`BindNode`/`BindActor`；
- 落库：变更先改内存并标记 dirty，500ms 内合并写库（`actor.AfterInvoke`），`Destroy` 同步落库；
- 下线：`Disconnect` 事件中 `Kill` 玩家 Actor，触发销毁落库。

已知限制：

- 500ms 为首次变更到保存调度的延迟，队列/数据库阻塞或写库失败会扩大未保存窗口；
- 跨 logic 实例重复登录尚未实现归属接管，版本条件可阻止覆盖，但不能替代会话互斥；
- 等级表示例提供 1–100 级的升级经验，超出配置范围保留经验并停止升级；
- Actor 销毁不等待在途 handler，并发读写依赖后续修复（详见评审结论）。

## 7. 运行与验证

```bash
docker compose up -d          # etcd + redis + mongo（本机已有 mongo 时不要重复启动）
go run ./cmd/backend
go run ./cmd/logic
go run ./cmd/battle
go run ./cmd/gate
go run ./cmd/client
```

预期输出：

```text
login ok (from mongo): uid=1001 nickname=player-1001 level=1 exp=0 gold=0
player data: uid=1001 nickname=player-1001 level=1 exp=0 gold=0
add exp +50: uid=1001 nickname=player-1001 level=1 exp=50 gold=0
add gold +100: uid=1001 nickname=player-1001 level=1 exp=50 gold=100
```

重启 logic 后再次登录，应看到 `level/exp/gold` 从 MongoDB 恢复；加经验达到阈值时自动升级。

## 8. 变更记录

| 日期 | 变更 |
| --- | --- |
| 2026-11-15 | 初版：玩家数据落 MongoDB，玩家内部功能改为 Actor 处理 |
| 2026-11-16 | 补充功能文档 |
| 2026-11-16 | TCP 协议迁移至 Protobuf，等级经验接入 Luban 表；同步版本保存与配置键说明 |
| 2026-11-16 | 同文档增加背包、任务及初始化标记，旧玩家上线补齐，快照追加 items/tasks |
