// Package protocol 定义客户端、网关与节点之间的路由号与消息结构。
//
// 路由号按业务域分段，一经上线就属于协议契约：新增路由请在自己的号段内追加，
// 不要复用或调整已有编号。客户端（含其他语言实现）必须与服务端保持一致。
package protocol

// 路由号
const (
	// RouteLogin 登录：校验 token（调用 backend）、从 MongoDB 加载玩家数据、绑定 UID 并创建玩家 Actor（无状态）
	RouteLogin int32 = 1
	// RoutePing 连通性测试（无状态）
	RoutePing int32 = 2

	// RoutePlayerData 查询玩家数据（有状态，由玩家 Actor 处理）
	RoutePlayerData int32 = 110
	// RouteAddExp 增加经验（有状态，由玩家 Actor 处理，自动升级）
	RouteAddExp int32 = 111
	// RouteAddGold 增加金币（有状态，由玩家 Actor 处理）
	RouteAddGold int32 = 112

	// RouteEnterBattle 进入战斗（无状态，需已登录；battle 节点负责绑定 UID）
	RouteEnterBattle int32 = 201
	// RouteAttack 发起攻击（有状态：按 UID 固定到 battle 实例）
	RouteAttack int32 = 202
	// RouteLeaveBattle 离开战斗（有状态）
	RouteLeaveBattle int32 = 203
)
