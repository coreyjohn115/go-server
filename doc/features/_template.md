# <功能名称>

> 状态：进行中 / 已实现
> 涉及进程：gate / logic / battle / backend / client
> 负责人：<name>
> 更新日期：YYYY-MM-DD

## 1. 功能概述

一句话说明功能目标、使用场景与边界（不做什么）。

## 2. 路由与接口

| 路由号 / 路径 | 类型 | 处理进程 | 说明 |
| --- | --- | --- | --- |
| `1` | 无状态 | logic | 示例：登录 |
| `110` | 有状态 + Actor | logic | 示例：玩家数据 |

## 3. 协议示例

请求：

```json
{}
```

响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

错误码：

| code | 含义 | 触发条件 |
| --- | --- | --- |
| 3 | 参数错误 | 参数缺失或非法 |

## 4. 数据与存储

- MongoDB：集合 `due.xxx`，字段说明
- Redis：key 格式与生命周期
- 内存状态：由哪个 Actor / 实例持有，重启是否丢失

## 5. 配置项

| 配置 | 位置 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `xxx.uri` | `config/app.toml` | `...` | 连接地址 |

## 6. 关键实现

- 路由分层：`internal/xxx/handler` → `internal/xxx/app` 注册
- Actor / Task / 队列与并发模型
- 落库策略与失败处理
- 上线/下线、超时、限流等边界

## 7. 运行与验证

启动顺序与命令：

```bash
docker compose up -d
go run ./cmd/backend
go run ./cmd/logic
go run ./cmd/battle
go run ./cmd/gate
go run ./cmd/client
```

预期输出：

```text
...
```

## 8. 变更记录

| 日期 | 变更 |
| --- | --- |
| YYYY-MM-DD | 初版 |
