// Command logic 启动 logic 节点（玩家逻辑服）。
//
// logic 是有状态节点：玩家登录成功后 UID 绑定到某个 logic 实例，
// 之后的玩家数据、经验、金币等有状态路由都会固定落到该实例。
//
// 玩家数据保存在 MongoDB：登录时加载（不存在则创建），内部系统功能由玩家 Actor
// 串行处理，数据变更后合并写回数据库。
package main

import (
	"context"

	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/node"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/log"

	"due-server/internal/common/backendapi"
	"due-server/internal/common/bootstrap"
	"due-server/internal/common/gamedata"
	"due-server/internal/logic/app"
	logichandler "due-server/internal/logic/handler"
	"due-server/internal/logic/store/playerstore"
)

func main() {
	// 初始化业务配置中心（./config）
	bootstrap.Init()
	tables, err := gamedata.Load(config.Get("app.tables.dir", "config/generated").String())
	if err != nil {
		log.Fatalf("load game tables failed: %v", err)
	}

	// backend 是独立的无状态 HTTP 服务，登录校验通过它完成
	backend := backendapi.NewClient(
		config.Get("app.backend.base_url", "http://127.0.0.1:8080").String(),
	)

	// 玩家数据存储（MongoDB）
	store, err := playerstore.New(
		context.Background(),
		config.Get("app.mongo.uri", "mongodb://127.0.0.1:27018").String(),
		config.Get("app.mongo.database", "due").String(),
		config.Get("app.mongo.collection", "players").String(),
	)
	if err != nil {
		log.Fatalf("connect mongo failed: %v", err)
	}

	container := due.NewContainer()

	component := node.NewNode(
		// 节点名称即服务组名，有状态路由按「UID + 服务组」定位实例
		node.WithName("logic"),
		node.WithLocator(bootstrap.Locator()),
		node.WithRegistry(bootstrap.Registry()),
	)

	app.RegisterLogic(component.Proxy(), logichandler.New(backend, store, tables))

	container.Add(component)
	container.Serve()

	if err = store.Close(context.Background()); err != nil {
		log.Warnf("close mongo failed: %v", err)
	}
}
