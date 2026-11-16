// Command client 是调试用客户端，按顺序演示完整链路：
//
//	连接网关 -> 登录（backend 校验 + MongoDB 加载数据）-> 查询玩家数据
//	-> 加经验 -> 加金币 -> 再查数据 -> 进入战斗 -> 攻击 x3 -> 离开战斗 -> 定时 ping
//
// 第二次启动时可以看到等级/经验/金币是从 MongoDB 恢复的；日志里的 node 字段
// 可以观察有状态路由是否始终落在同一个实例上。
package main

import (
	"time"

	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/cluster/client"
	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/log"

	"due-server/internal/common/bootstrap"
	"due-server/internal/common/protocol"
)

const (
	// 调试用户 ID
	demoUID = int64(1001)
	// ping 间隔
	pingInterval = 5 * time.Second
	// 攻击次数
	attackTimes = 3
)

type demo struct {
	step    int
	attacks int
}

func main() {
	// 初始化业务配置中心（读取 app.token）
	bootstrap.Init()

	container := due.NewContainer()

	component := client.NewClient(
		client.WithClient(tcp.NewClient()),
	)

	initListeners(component.Proxy(), &demo{})

	container.Add(component)
	container.Serve()
}

func initListeners(proxy *client.Proxy, d *demo) {
	// 组件启动后拨号
	proxy.AddHookListener(cluster.Start, onStart)
	// 连接建立后发起登录
	proxy.AddEventListener(cluster.Connect, onConnect)

	// 注册各路由的回包处理
	proxy.AddRouteHandler(protocol.RouteLogin, d.onLogin)
	proxy.AddRouteHandler(protocol.RoutePing, d.onPing)
	proxy.AddRouteHandler(protocol.RoutePlayerData, d.onPlayerData)
	proxy.AddRouteHandler(protocol.RouteAddExp, d.onAddExp)
	proxy.AddRouteHandler(protocol.RouteAddGold, d.onAddGold)
	proxy.AddRouteHandler(protocol.RouteEnterBattle, d.onEnterBattle)
	proxy.AddRouteHandler(protocol.RouteAttack, d.onAttack)
	proxy.AddRouteHandler(protocol.RouteLeaveBattle, d.onLeaveBattle)
}

func onStart(proxy *client.Proxy) {
	if _, err := proxy.Dial(); err != nil {
		log.Errorf("dial gate failed: %v", err)
	}
}

func onConnect(conn *client.Conn) {
	log.Info("connected to gate")
	push(conn, protocol.RouteLogin, &protocol.LoginReq{
		UID:   demoUID,
		Token: config.Get("app.app.token", "dev-token").String(),
	})
}

func (d *demo) onLogin(ctx *client.Context) {
	res := &protocol.LoginRes{}

	if err := ctx.Parse(res); err != nil {
		log.Errorf("parse login response failed: %v", err)
		return
	}
	if res.Code != int32(codes.OK.Code()) {
		log.Errorf("login failed, code: %d, message: %s", res.Code, res.Message)
		return
	}

	logPlayer("login ok (from mongo)", res.Player, res.Node)

	// 有状态路由 + Actor：查询玩家数据
	push(ctx.Conn(), protocol.RoutePlayerData, &protocol.PlayerDataReq{})
}

func (d *demo) onPlayerData(ctx *client.Context) {
	res := &protocol.PlayerDataRes{}

	if err := ctx.Parse(res); err != nil {
		log.Errorf("parse player data response failed: %v", err)
		return
	}
	if res.Code != int32(codes.OK.Code()) {
		log.Errorf("player data failed, code: %d, message: %s", res.Code, res.Message)
		return
	}

	logPlayer("player data", res.Player, res.Node)

	if d.step == 0 {
		d.step = 1
		// 玩家内部系统功能：加经验（Actor 处理，自动升级）
		push(ctx.Conn(), protocol.RouteAddExp, &protocol.AddExpReq{Delta: 50})
		return
	}

	// 加经验、加金币之后再次进入战斗流程
	push(ctx.Conn(), protocol.RouteEnterBattle, &protocol.EnterBattleReq{Mode: "1v1"})
}

func (d *demo) onAddExp(ctx *client.Context) {
	res := &protocol.AddExpRes{}

	if err := ctx.Parse(res); err != nil {
		log.Errorf("parse add exp response failed: %v", err)
		return
	}
	if res.Code != int32(codes.OK.Code()) {
		log.Errorf("add exp failed, code: %d, message: %s", res.Code, res.Message)
		return
	}

	logPlayer("add exp +50", res.Player, res.Node)

	push(ctx.Conn(), protocol.RouteAddGold, &protocol.AddGoldReq{Delta: 100})
}

func (d *demo) onAddGold(ctx *client.Context) {
	res := &protocol.AddGoldRes{}

	if err := ctx.Parse(res); err != nil {
		log.Errorf("parse add gold response failed: %v", err)
		return
	}
	if res.Code != int32(codes.OK.Code()) {
		log.Errorf("add gold failed, code: %d, message: %s", res.Code, res.Message)
		return
	}

	logPlayer("add gold +100", res.Player, res.Node)

	// 再查一次，验证内存中的变更已经生效（稍后会合并写库）
	push(ctx.Conn(), protocol.RoutePlayerData, &protocol.PlayerDataReq{})
}

func (d *demo) onEnterBattle(ctx *client.Context) {
	res := &protocol.EnterBattleRes{}

	if err := ctx.Parse(res); err != nil {
		log.Errorf("parse enter battle response failed: %v", err)
		return
	}
	if res.Code != int32(codes.OK.Code()) {
		log.Errorf("enter battle failed, code: %d, message: %s", res.Code, res.Message)
		return
	}

	log.Infof("enter battle ok, room: %s, battle node: %s", res.RoomID, res.Node)

	d.attacks = 1
	push(ctx.Conn(), protocol.RouteAttack, &protocol.AttackReq{Skill: "normal"})
}

func (d *demo) onAttack(ctx *client.Context) {
	res := &protocol.AttackRes{}

	if err := ctx.Parse(res); err != nil {
		log.Errorf("parse attack response failed: %v", err)
		return
	}
	if res.Code != int32(codes.OK.Code()) {
		log.Errorf("attack failed, code: %d, message: %s", res.Code, res.Message)
		return
	}

	log.Infof("attack ok, room: %s, hits: %d, battle node: %s", res.RoomID, res.Hits, res.Node)

	if d.attacks < attackTimes {
		d.attacks++
		push(ctx.Conn(), protocol.RouteAttack, &protocol.AttackReq{Skill: "normal"})
		return
	}

	push(ctx.Conn(), protocol.RouteLeaveBattle, &protocol.LeaveBattleReq{})
}

func (d *demo) onLeaveBattle(ctx *client.Context) {
	res := &protocol.LeaveBattleRes{}

	if err := ctx.Parse(res); err != nil {
		log.Errorf("parse leave battle response failed: %v", err)
		return
	}
	if res.Code != int32(codes.OK.Code()) {
		log.Errorf("leave battle failed, code: %d, message: %s", res.Code, res.Message)
		return
	}

	log.Infof("leave battle ok, room: %s, battle node: %s", res.RoomID, res.Node)

	schedulePing(ctx.Conn())
}

func (d *demo) onPing(ctx *client.Context) {
	res := &protocol.PingRes{}

	if err := ctx.Parse(res); err != nil {
		log.Errorf("parse ping response failed: %v", err)
		return
	}
	if res.Code != int32(codes.OK.Code()) {
		log.Errorf("ping failed, code: %d, message: %s", res.Code, res.Message)
		return
	}

	log.Infof("pong received: %s, server time: %s, logic node: %s", res.Message, res.ServerTime, res.Node)

	schedulePing(ctx.Conn())
}

func logPlayer(tag string, p *protocol.Player, node string) {
	if p == nil {
		log.Infof("%s: empty player, logic node: %s", tag, node)
		return
	}

	log.Infof("%s: uid=%d nickname=%s level=%d exp=%d gold=%d, logic node=%s",
		tag, p.UID, p.Nickname, p.Level, p.Exp, p.Gold, node)
}

func schedulePing(conn *client.Conn) {
	time.AfterFunc(pingInterval, func() {
		push(conn, protocol.RoutePing, &protocol.PingReq{Message: "hello due"})
	})
}

func push(conn *client.Conn, route int32, data any) {
	if err := conn.Push(&cluster.Message{Route: route, Data: data}); err != nil {
		log.Errorf("push message failed, route: %d, err: %v", route, err)
	}
}
