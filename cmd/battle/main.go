// Command battle 启动 battle 节点（战斗服）。
//
// battle 是有状态节点：进入战斗时 UID 绑定到某个 battle 实例，
// 攻击、离开战斗等有状态路由都会回到该实例，房间状态保存在实例内存中。
package main

import (
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/node"

	"due-server/internal/battle/app"
	battlehandler "due-server/internal/battle/handler"
	"due-server/internal/common/bootstrap"
)

func main() {
	bootstrap.Init()

	container := due.NewContainer()

	component := node.NewNode(
		node.WithName("battle"),
		node.WithLocator(bootstrap.Locator()),
		node.WithRegistry(bootstrap.Registry()),
	)

	app.RegisterBattle(component.Proxy(), battlehandler.New())

	container.Add(component)
	container.Serve()
}
