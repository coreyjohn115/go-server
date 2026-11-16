# 项目文档

本目录存放项目功能文档。**新增或修改功能时，必须同步更新对应文档**（项目 skill：`feature-docs`）。

## 目录约定

| 路径 | 说明 |
| --- | --- |
| `doc/features/` | 一个功能一个文件，小写 kebab-case 命名 |
| `doc/features/_template.md` | 新功能文档模板 |
| `doc/README.md` | 本文档，维护功能索引 |

## 功能索引

| 功能 | 文档 | 涉及进程 | 状态 |
| --- | --- | --- | --- |
| 玩家数据（MongoDB + Actor） | [features/player-data.md](features/player-data.md) | logic | 已实现 |
| 道具与背包 | [features/items.md](features/items.md) | logic/client | 已实现 |
| 玩家任务 | [features/tasks.md](features/tasks.md) | logic/client | 已实现 |
| Protobuf 与 Luban 生成 | [features/code-generation.md](features/code-generation.md) | logic/battle/client/robot | 已实现 |

## 如何新增功能文档

1. 复制 `doc/features/_template.md` 为 `doc/features/<feature>.md`；
2. 按模板填写功能概述、路由/接口、协议示例、数据、配置、实现、验证；
3. 在上面的"功能索引"表追加一行；
4. 与代码放在同一次改动中提交。

> 纯重构、格式化、注释调整不需要新建文档；修复 bug 不改变行为时，更新原文档的"变更记录"即可。
