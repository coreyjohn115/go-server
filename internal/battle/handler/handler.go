// Package handler 实现 battle 节点（战斗服）的业务处理器。
//
// battle 是状态节点：玩家进入战斗时 UID 绑定到某个 battle 实例，
// 之后攻击、离开等有状态路由都会固定落到该实例，房间数据也只保存在该实例内。
package handler

import (
	"fmt"
	"sync"
	"time"

	"github.com/dobyte/due/v2/cluster/node"
	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xtime"

	"due-server/internal/common/protocol"
)

// Room 是一场战斗的最小状态。
type Room struct {
	ID        string
	UID       int64
	Mode      string
	Hits      int
	CreatedAt time.Time
}

// Battle 是 battle 节点的业务处理器集合，同时保存本节点的房间状态。
type Battle struct {
	mu    sync.Mutex
	rooms map[int64]*Room
}

// New 创建 battle 处理器。
func New() *Battle {
	return &Battle{rooms: make(map[int64]*Room)}
}

// Enter 进入战斗：把 UID 绑定到当前 battle 实例，并创建（或复用）房间。
func (h *Battle) Enter(ctx node.Context) {
	req := &protocol.EnterBattleReq{}
	res := &protocol.EnterBattleRes{Code: int32(codes.InternalError.Code())}

	defer func() {
		if err := ctx.Response(res); err != nil {
			log.Errorf("response enter battle failed: %v", err)
		}
	}()

	if err := ctx.Parse(req); err != nil {
		log.Errorf("parse enter battle request failed: %v", err)
		res.Code = int32(codes.InvalidArgument.Code())
		return
	}

	uid := ctx.UID()
	if uid <= 0 {
		res.Code = int32(codes.Unauthorized.Code())
		return
	}

	// 绑定后，battle 的有状态路由（攻击/离开）都会定位到本实例。
	if err := ctx.BindNode(uid); err != nil {
		log.Errorf("bind battle node failed, uid: %d, err: %v", uid, err)
		return
	}

	h.mu.Lock()
	room, ok := h.rooms[uid]
	if !ok {
		room = &Room{
			ID:        fmt.Sprintf("room-%d", uid),
			UID:       uid,
			Mode:      req.Mode,
			CreatedAt: xtime.Now(),
		}
		h.rooms[uid] = room
	}
	h.mu.Unlock()

	log.Infof("user entered battle, uid: %d, room: %s, node: %s", uid, room.ID, ctx.Proxy().GetID())

	res.Code = int32(codes.OK.Code())
	res.Message = "enter battle ok"
	res.RoomID = room.ID
	res.UID = uid
	res.Node = ctx.Proxy().GetID()
}

// Attack 发起攻击：有状态路由，房间状态只在本实例内存中。
func (h *Battle) Attack(ctx node.Context) {
	req := &protocol.AttackReq{}
	res := &protocol.AttackRes{Code: int32(codes.InternalError.Code())}

	defer func() {
		if err := ctx.Response(res); err != nil {
			log.Errorf("response attack failed: %v", err)
		}
	}()

	if err := ctx.Parse(req); err != nil {
		log.Errorf("parse attack request failed: %v", err)
		res.Code = int32(codes.InvalidArgument.Code())
		return
	}

	uid := ctx.UID()

	h.mu.Lock()
	room, ok := h.rooms[uid]
	if ok {
		room.Hits++
		res.Hits = int32(room.Hits)
		res.RoomID = room.ID
	}
	h.mu.Unlock()

	if !ok {
		res.Code = int32(codes.NotFound.Code())
		res.Message = "player is not in battle"
		return
	}

	log.Infof("attack handled, uid: %d, skill: %s, hits: %d, node: %s", uid, req.Skill, res.Hits, ctx.Proxy().GetID())

	res.Code = int32(codes.OK.Code())
	res.Message = "attack ok"
	res.Node = ctx.Proxy().GetID()
}

// Leave 离开战斗：有状态路由，清理房间并解除 battle 绑定。
func (h *Battle) Leave(ctx node.Context) {
	req := &protocol.LeaveBattleReq{}
	res := &protocol.LeaveBattleRes{Code: int32(codes.InternalError.Code())}

	defer func() {
		if err := ctx.Response(res); err != nil {
			log.Errorf("response leave battle failed: %v", err)
		}
	}()

	if err := ctx.Parse(req); err != nil {
		log.Errorf("parse leave battle request failed: %v", err)
		res.Code = int32(codes.InvalidArgument.Code())
		return
	}

	uid := ctx.UID()

	h.mu.Lock()
	room, ok := h.rooms[uid]
	if ok {
		res.RoomID = room.ID
	}
	h.mu.Unlock()

	// 解绑成功后再清理房间；失败时保留状态，允许客户端重试。
	if err := ctx.UnbindNode(uid); err != nil {
		log.Errorf("unbind battle node failed, uid: %d, err: %v", uid, err)
		return
	}
	h.mu.Lock()
	delete(h.rooms, uid)
	h.mu.Unlock()
	if ok {
		log.Infof("user left battle, uid: %d, room: %s", uid, res.RoomID)
	}

	res.Code = int32(codes.OK.Code())
	res.Message = "leave battle ok"
	res.Node = ctx.Proxy().GetID()
}
