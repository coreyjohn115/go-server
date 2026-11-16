# 道具与背包

## 功能概述

参考 Game101 的 ItemComponent、InitItemConfig 和 ItemMode，在玩家 Actor 内实现初始发放、堆叠、使用、删除和道具事件。金币奖励直接计入 `gold`，不额外保存金币堆栈。

## 路由与协议

以下均为 logic 有状态 + Authorized 路由，必须先登录绑定 Actor。TCP 使用 Protobuf，JSON 仅说明字段。

| 路由 | 请求 | 响应 |
| --- | --- | --- |
| 120 | ItemListReq `{}` | ItemListRes `{"code":0,"items":[{"id":1,"configId":1002,"count":3}]}` |
| 121 | UseItemReq `{"id":1,"count":1}` | ItemChangeRes |
| 122 | DeleteItemReq `{"id":1,"count":1}` | ItemChangeRes |

使用/删除按堆栈实例 ID 操作。成功返回 `{"code":0,"player":{...完整最新玩家快照...}}`，包含 items/tasks；客户端整体替换快照即可处理堆栈删除及任务变化。登录和玩家查询也追加返回这两个字段。

无效数量、道具不足、不可使用、效果溢出、容量不足、版本冲突返回 due InvalidArgument（3）及 message，不返回成功快照。未登录由网关/Actor 转发层拒绝。每次数量 1–1,000,000，背包最多 10,000 堆栈。客户端没有任意增加资产的路由，奖励从 Actor 内部 `addItem` 发放。

## 配置项

修改 Excel 后执行 `gen.bat`，提交源表及生成 Go/JSON，重启 logic 生效；本次静态表不支持热更新。

| 源表 | 生成 JSON | 字段 |
| --- | --- | --- |
| excel/Data/item.xlsx | tbitem.json | id/name/maxStack |
| excel/Data/item_rule.xlsx | tbitemrule.json | id/mode/value/rewardItem/rewardCount |
| excel/Data/init_item.xlsx | tbinititem.json | id/count |

mode：0 不可用，1 金币，2 经验，3 纯消耗，4 礼包。value 为每份金币/经验，礼包按 rewardCount × 使用数量发放 rewardItem，不递归自动使用。金币道具奖励自动记入 gold，旧背包金币堆栈可兑换。

默认初始化：1002 药水 3 瓶（每瓶 50 经验）、1003 药水礼包 1 个（内含药水 2 瓶）、120001 洗牌 3 个。120001 沿用 Game101 的 ID 和纯扣除方式，棋盘效果需游戏客户端实现。头像、头像框、活动和掉落池依赖尚未移植，因此不直接复制 Game101 的全量配置。

## 数据与存储

MongoDB `due.players` 新增 `items:[{id,configId,count}]`、`nextItemId`、`initItemIds`。堆栈 ID 在玩家内单调递增，填满旧堆栈再新建。初始发放按配置 ID 留标记，旧玩家首次上线自动补齐；新增配置 ID 下次上线补发，修改已有行数量不补发。没有额外 Redis key。配置移除的旧道具保留，但不能使用/删除，需专门迁移。

## 关键实现

同一玩家由 Actor 串行处理；使用、扣除、任务进度和奖励全部在副本上计算，任何失败整体回滚，成功才更新内存并递增一次 version。道具与任务复用同一 Mongo 文档版本条件更新、500ms 合并写入、销毁保存。成功响应尚不代表已落盘，强杀可能回退未保存变化；跨实例重复登录归属仍受现有系统限制。

## 运行与验证

`docker compose up -d` 后分别运行 `go run ./cmd/backend`、`go run ./cmd/logic`、`go run ./cmd/battle`、`go run ./cmd/gate`，再运行 `go run ./cmd/client`，登录后输出 inventory/tasks。

`go test ./internal/logic/actor/player ./internal/common/gamedata ./internal/common/protocol` 验证初始化、堆叠、使用/删除、礼包、任务、溢出、失败回滚和 BSON 恢复。全量检查：`go fmt ./...`、`go vet ./...`、`go test ./...`、`go build ./...`。

## 变更记录

- 2026-11-16：新增道具系统、Luban 初始化/效果配置和 Protobuf 协议。
