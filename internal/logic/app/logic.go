// Package app 负责把 logic 节点的业务 handler、路由与事件装配到 due 组件上。
package app

import (
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/cluster/node"

	"due-server/internal/common/protocol"
	"due-server/internal/logic/handler"
)

// RegisterLogic 注册 logic 节点（玩家逻辑服）的路由与事件。
//
// 登录、ping 是无状态路由：网关按分发策略选择任意 logic 实例；
// 玩家内部系统功能（数据查询、加经验、加金币）是有状态路由，
// 由节点级 handler 转交给玩家 Actor 处理。
func RegisterLogic(proxy *node.Proxy, h *handler.Logic) {
	proxy.Router().AddRouteHandler(protocol.RouteLogin, h.Login)
	proxy.Router().AddRouteHandler(protocol.RoutePing, h.Ping)

	// 玩家 Actor 路由：节点只负责转交，具体处理在 Actor 内
	proxy.Router().AddRouteHandler(protocol.RoutePlayerData, h.NextToActor,
		node.RouteOptions{Stateful: true, Authorized: true})
	proxy.Router().AddRouteHandler(protocol.RouteAddExp, h.NextToActor,
		node.RouteOptions{Stateful: true, Authorized: true})
	proxy.Router().AddRouteHandler(protocol.RouteAddGold, h.NextToActor,
		node.RouteOptions{Stateful: true, Authorized: true})

	// 连接事件：上线创建 Actor，下线销毁 Actor（销毁时同步落库）
	proxy.AddEventHandler(cluster.Connect, h.OnConnect)
	proxy.AddEventHandler(cluster.Disconnect, h.OnDisconnect)
}
