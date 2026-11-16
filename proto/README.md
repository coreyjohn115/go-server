# Protobuf 协议

`message.proto` 是 TCP 消息的唯一结构定义，生成结果为 `internal/common/protocol/message.pb.go`。
路由号继续维护在 `internal/common/protocol/route.go`，已有编号保持不变。

在任意目录执行本目录的 `gen.bat`，或在项目根目录执行：

```bat
py scripts/generate.py proto
```

需要 Windows、Python 3、Go、curl。首次自动下载 protoc 33.0，并从 go.mod 固定的 protobuf v1.36.12 构建 Go 插件，缓存于 `.codex-work/tools/`。可用 `PROTOC` 指定已有编译器。

不要手改 `.pb.go`。新增字段追加新编号；删除字段必须保留编号（`reserved`），不可复用。
`UID`、`RoomID` 保留现有 Go 字段拼写；错误码和攻击计数采用 int32。
消息字段号与路由号是两类独立契约。客户端与服务端必须同时升级为 Protobuf，旧 JSON TCP 客户端不兼容。
backend HTTP 契约继续使用 JSON。
