# Luban 配置工程

- `Data/player_level.xlsx`：等级与升级所需经验，示例等级 1–100。
- `Data/item.xlsx`：道具 ID、名称、堆叠上限。
- `Defines/game.xml`：Bean、Table、索引与输入文件定义。
- `luban.conf`：服务端导出目标，Go 包为 `gamedata`。

修改 Excel 后关闭文件，再执行本目录 `gen.bat`，或在项目根目录运行：

```bat
py scripts/generate.py config
```

首次需要联网下载 Luban 5.1.0 和解压工具，需要 Python 3、curl、Go（gofmt）与 .NET 8 或更高版本对应运行时。
Luban 发布包会校验 SHA-256。工具缓存位于 `.codex-work/tools/`，可通过 `LUBAN_DLL` 指定已有工具。
日常编辑和导出不需要 Python Excel 库；示例工作簿已入库。

输出包括 `internal/common/gamedata/` 的生成类型与 `config/generated/` 的 JSON 数据，均应提交版本库。
不要修改生成文件；自定义加载、校验代码位于 `load.go`。新增表必须同时增加 XML 定义、Excel 数据并重新生成。
Excel 第一列是 Luban 行标记，前四行分别为 `##var`、`##type`、`##group`、注释；数据从第五行开始。

logic 启动从 `app.tables.dir` 加载并校验整套表，默认 `config/generated`。等级必须从 1 连续排列，经验为正数。
表数据只读，修改后重新导出并重启 logic；到达最后一行等级的下一级后不再升级，经验保留。
数据库地址、token 等部署参数仍使用 `config/app.toml`，不放入策划表。
