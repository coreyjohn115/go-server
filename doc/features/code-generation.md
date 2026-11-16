# 协议与配置生成

## 功能概述

TCP 消息使用 Protobuf；静态游戏配置使用 Luban 从 Excel 生成 Go 类型与 JSON 数据。
根目录 `gen.bat` 生成全部，`proto/gen.bat` 生成协议，`Excel/gen.bat` 导表。

## 路由与接口

已有路由 1、2、110–112、201–203 不变。logic/battle/client/robot 的消息均使用生成类型；
`etc/etc.toml` 的 node/client codec 为 `proto`。gate 透传消息，packet 参数保持一致。
backend HTTP 仍使用 JSON，路径与契约不变。

## 协议示例

登录对应 `LoginReq { UID: 1001, token: "dev-token" }`；响应为 `LoginRes`，字段包含 code、message、player、node。
以上为字段示意，实际 TCP 数据是 Protobuf 二进制，错误码沿用 due codes。
消息编号见 `proto/message.proto`，追加字段不可复用已有字段号。

## 数据与存储

Excel 源数据在 `Excel/Data`，定义在 `Excel/Defines`；生成类型在 `internal/common/gamedata`，
运行数据在 `config/generated`。logic 在启动时完整加载并校验，作为只读快照传给玩家 Processor。
升级优先使用等级表，超出配置范围停止升级并保留经验。MongoDB、Redis 结构未改变。

## 配置项与关键实现

`config/app.toml` 的 `[tables] dir` 默认 `config/generated`，实际键为 `app.tables.dir`，启动时读取，不支持热更新。
部署时须携带导出的 JSON；文件缺失或非法将阻止 logic 启动。
工具固定为 protoc 33.0、protoc-gen-go v1.36.12、Luban 5.1.0，缓存在 `.codex-work/tools`。
环境变量 `PROTOC`、`LUBAN_DLL` 可指定本地工具。首次下载需联网，后续可复用缓存离线生成。

## 运行与验证

```bat
gen.bat
go test ./...
go vet ./...
go build ./...
```

预期两种生成器成功退出，协议往返测试、导出表加载测试通过。
启动流程沿用 start.bat，client/robot 与服务端必须一起切换；JSON TCP 客户端无法兼容此次变更。

## 变更记录

- 2026-11-16：初始化 Protobuf 与 Luban 工程、示例表、生成入口与等级配置加载。
