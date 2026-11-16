// Package app 负责把 battle 节点的业务 handler 装配到 due 组件上。
package app

import (
	"github.com/dobyte/due/v2/cluster/node"

	"due-server/internal/battle/handler"
	"due-server/internal/common/protocol"
)

// RegisterBattle 注册 battle 节点（战斗服）的路由。
//
// 进入战斗是无状态路由（任一 battle 实例都可以接），处理时把 UID 绑定到该实例；
// 攻击、离开战斗是有状态路由，始终回到玩家所在的 battle 实例。
func RegisterBattle(proxy *node.Proxy, h *handler.Battle) {
	proxy.Router().AddRouteHandler(protocol.RouteEnterBattle, h.Enter,
		node.RouteOptions{Authorized: true})
	proxy.Router().AddRouteHandler(protocol.RouteAttack, h.Attack,
		node.RouteOptions{Stateful: true, Authorized: true})
	proxy.Router().AddRouteHandler(protocol.RouteLeaveBattle, h.Leave,
		node.RouteOptions{Stateful: true, Authorized: true})
}
