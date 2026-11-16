// Command gate 启动网关进程。
//
// 网关只负责维护客户端连接与消息转发，不承载游戏业务逻辑；
// 业务消息会按路由号被转发到节点（cmd/logic、cmd/battle）。
package main

import (
	"github.com/dobyte/due/v2"

	"due-server/internal/gate"
)

func main() {
	container := due.NewContainer()

	container.Add(gate.New())

	container.Serve()
}
