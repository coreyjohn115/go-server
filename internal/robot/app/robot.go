// Package app 装配 robot 客户端的路由与事件。
package app

import (
	"due-server/internal/robot/handler"
	"github.com/dobyte/due/v2/cluster/client"
)

func RegisterRobot(proxy *client.Proxy, runner *handler.Runner) {
	runner.Register(proxy)
}
