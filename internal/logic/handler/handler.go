// Package handler 实现 logic 节点（玩家逻辑服）的业务处理器。
//
// logic 是状态节点：玩家登录后 UID 会绑定到某个 logic 实例，
// 之后所有有状态路由都会固定落到该实例。
//
// 玩家的内部系统功能（查数据、加经验、加金币）由「每个玩家一个 Actor」处理：
// 节点级路由只负责把消息转交给 Actor，真正的数据读写发生在 Actor 的串行执行域内。
package handler

import (
	"strconv"

	"github.com/dobyte/due/v2/cluster/node"
	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xtime"

	"due-server/internal/common/backendapi"
	"due-server/internal/common/gamedata"
	"due-server/internal/common/protocol"
	"due-server/internal/logic/actor/player"
	"due-server/internal/logic/store/playerstore"
)

// Logic 是 logic 节点的业务处理器集合。
type Logic struct {
	backend *backendapi.Client
	store   *playerstore.Store
	tables  *gamedata.Tables
}

// New 创建 logic 处理器。
func New(backend *backendapi.Client, store *playerstore.Store, tables *gamedata.Tables) *Logic {
	return &Logic{backend: backend, store: store, tables: tables}
}

// Login 处理登录请求：backend 校验令牌 -> MongoDB 加载玩家 -> 绑定 UID -> 创建玩家 Actor。
func (h *Logic) Login(ctx node.Context) {
	req := &protocol.LoginReq{}

	if err := ctx.Parse(req); err != nil {
		log.Errorf("parse login request failed: %v", err)
		h.response(ctx, &protocol.LoginRes{Code: int32(codes.InvalidArgument.Code())})
		return
	}

	uid, token := req.UID, req.Token

	// 登录包含 HTTP 调用与数据库读取，交给任务池执行，避免占用节点执行域。
	// Task 会接管 ctx 的所有权，因此回包必须写在任务内部，外层 handler 立即返回。
	ctx.Task(func(ctx node.Context) {
		res := &protocol.LoginRes{Code: int32(codes.InternalError.Code())}

		defer func() {
			if err := ctx.Response(res); err != nil {
				log.Errorf("response login failed: %v", err)
			}
		}()

		if uid <= 0 {
			res.Code = int32(codes.InvalidArgument.Code())
			return
		}

		data, err := h.backend.VerifyToken(ctx.Context(), uid, token)
		if err != nil {
			log.Errorf("verify token via backend failed: %v", err)
			return
		}
		if !data.OK {
			log.Warnf("login rejected, uid: %d", uid)
			res.Code = int32(codes.Unauthorized.Code())
			res.Message = "invalid token"
			return
		}

		if err = ctx.BindGate(uid); err != nil {
			log.Errorf("bind gate failed, uid: %d, err: %v", uid, err)
			return
		}
		if err = ctx.BindNode(uid); err != nil {
			log.Errorf("bind node failed, uid: %d, err: %v", uid, err)
			return
		}

		// 创建（或复用）玩家 Actor，并把 UID 绑定到 Actor
		actor, err := h.ensureActor(ctx, uid, data.Nickname)
		if err != nil {
			log.Errorf("ensure player actor failed, uid: %d, err: %v", uid, err)
			return
		}

		// 从 Actor 读取最新快照（等价于从数据库加载的数据，且包含内存中的最新变更）
		snap, err := snapshot(actor)
		if err != nil {
			log.Errorf("read player snapshot failed, uid: %d, err: %v", uid, err)
			return
		}

		log.Infof("user logged in, uid: %d, cid: %d, node: %s", uid, ctx.CID(), ctx.Proxy().GetID())

		res.Code = int32(codes.OK.Code())
		res.Message = "login ok"
		res.Player = snap
		res.Node = ctx.Proxy().GetID()
	})
}

// NextToActor 把玩家内部系统功能的消息转交给玩家 Actor 处理。
//
// 节点级路由只做转交：ctx.Next() 会按「UID + Actor 类型」找到绑定到该玩家的 Actor，
// 之后的解析、数据处理、回包都在 Actor 的串行执行域中完成。
func (h *Logic) NextToActor(ctx node.Context) {
	if err := ctx.Next(); err != nil {
		log.Errorf("dispatch to player actor failed, uid: %d, route: %d, err: %v", ctx.UID(), ctx.Route(), err)

		code := codes.Convert(err)
		if e := ctx.Response(&protocol.ErrorRes{Code: int32(code.Code()), Message: code.Message()}); e != nil {
			log.Errorf("response dispatch error failed: %v", e)
		}
	}
}

// Ping 处理连通性测试请求。
func (h *Logic) Ping(ctx node.Context) {
	req := &protocol.PingReq{}
	res := &protocol.PingRes{Code: int32(codes.InternalError.Code())}

	defer func() {
		if err := ctx.Response(res); err != nil {
			log.Errorf("response ping failed: %v", err)
		}
	}()

	if err := ctx.Parse(req); err != nil {
		log.Errorf("parse ping request failed: %v", err)
		res.Code = int32(codes.InvalidArgument.Code())
		return
	}

	res.Code = int32(codes.OK.Code())
	res.Message = "pong: " + req.Message
	res.ServerTime = xtime.Now().Format(xtime.DateTime)
	res.Node = ctx.Proxy().GetID()
}

// OnConnect 处理连接建立事件。
func (h *Logic) OnConnect(ctx node.Context) {
	log.Infof("connection established, cid: %d, uid: %d", ctx.CID(), ctx.UID())
}

// OnDisconnect 处理连接断开事件：销毁玩家 Actor，Actor 销毁时会同步落库。
func (h *Logic) OnDisconnect(ctx node.Context) {
	uid := ctx.UID()

	log.Infof("connection closed, cid: %d, uid: %d", ctx.CID(), uid)

	if uid <= 0 {
		return
	}

	if !ctx.Proxy().Kill(playeractor.Kind, strconv.FormatInt(uid, 10)) {
		log.Warnf("kill player actor failed or not exists, uid: %d", uid)
	}
}

// ensureActor 获取或创建玩家 Actor，并把 UID 绑定到它。
func (h *Logic) ensureActor(ctx node.Context, uid int64, nickname string) (*node.Actor, error) {
	id := strconv.FormatInt(uid, 10)
	proxy := ctx.Proxy()

	actor, ok := proxy.Actor(playeractor.Kind, id)
	if !ok {
		// 首次上线：从 MongoDB 加载玩家数据（不存在则创建初始数据）
		data, err := h.store.LoadOrCreate(ctx.Context(), uid, nickname)
		if err != nil {
			return nil, err
		}

		actor, err = proxy.Spawn(func(actor *node.Actor, _ ...any) node.Processor {
			return playeractor.New(actor, h.store, data, h.tables)
		}, node.WithActorKind(playeractor.Kind), node.WithActorID(id))
		if err != nil {
			// 并发登录时可能已被创建，回读一次
			if actor, ok = proxy.Actor(playeractor.Kind, id); !ok {
				return nil, err
			}
		}
	}

	// 等待 Actor 执行完已排队的任务（包括路由注册），
	// 避免登录成功后客户端立即请求时路由尚未注册。
	if err := actor.Invoke(func() {}, true); err != nil {
		return nil, err
	}

	if err := proxy.BindActor(uid, playeractor.Kind, id); err != nil {
		return nil, err
	}

	return actor, nil
}

// snapshot 在 Actor 执行域内读取玩家数据快照。
func snapshot(actor *node.Actor) (*protocol.Player, error) {
	var snap *protocol.Player

	err := actor.Invoke(func() {
		if processor, ok := actor.Processor().(*playeractor.Processor); ok {
			snap = processor.Snapshot()
		}
	}, true)

	return snap, err
}

func (h *Logic) response(ctx node.Context, res *protocol.LoginRes) {
	if err := ctx.Response(res); err != nil {
		log.Errorf("response login failed: %v", err)
	}
}
