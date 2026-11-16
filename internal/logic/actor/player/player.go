// Package playeractor 实现玩家 Actor 的业务处理器。
//
// Actor 本身（消息队列、任务队列、串行调度、路由分发、生命周期）由 due 的
// cluster/node 包提供，本包只实现它的业务 Processor（node.Processor）：
// 每个在线玩家对应一个 Actor（kind=player，id=UID），
// 玩家的内部系统功能（查数据、加经验、加金币……）都作为 Actor 消息串行处理，
// 因此同一玩家的数据读写不需要额外加锁。
//
// 数据变更先更新内存，再延迟合并写回 MongoDB：短时间内多次变更只落库一次，
// 减少数据库压力；Actor 销毁（下线/停服）时会尝试同步落库。
package playeractor

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/dobyte/due/v2/cluster/node"
	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/log"

	"due-server/internal/common/protocol"
	"due-server/internal/logic/store/playerstore"
)

// Kind 是玩家 Actor 的类型
const Kind = "player"

const (
	// expPerLevel 是升到下一级所需的经验基数
	expPerLevel int64 = 100
	// saveDelay 合并写库的延迟
	saveDelay = 500 * time.Millisecond
	// saveTimeout 单次写库超时
	saveTimeout       = 3 * time.Second
	maxDelta    int64 = 1_000_000
)

type playerStore interface {
	Save(context.Context, *playerstore.Player, int64) error
}

type saveTimer interface{ Stop() bool }

// Processor 是玩家 Actor 的业务处理器，实现 node.Processor。
type Processor struct {
	actor        *node.Actor
	store        playerStore
	data         *playerstore.Player
	dirty        bool
	timer        saveTimer
	savedVersion int64
	closing      bool
	conflict     bool
	schedule     func(time.Duration, func()) (saveTimer, error)
}

// New 创建玩家 Actor 的业务处理器。
func New(actor *node.Actor, store *playerstore.Store, data *playerstore.Player) *Processor {
	return &Processor{actor: actor, store: store, data: data, savedVersion: data.Version,
		schedule: func(d time.Duration, f func()) (saveTimer, error) { return actor.AfterInvoke(d, f) },
	}
}

// Init 注册玩家内部系统功能的路由。
func (a *Processor) Init() {
	a.actor.AddRouteHandler(protocol.RoutePlayerData, a.handlePlayerData)
	a.actor.AddRouteHandler(protocol.RouteAddExp, a.handleAddExp)
	a.actor.AddRouteHandler(protocol.RouteAddGold, a.handleAddGold)
}

// Start 是 Actor 启动回调。
func (a *Processor) Start() {}

// Destroy 是 Actor 销毁回调：停止延迟写库并同步落库一次。
func (a *Processor) Destroy() {
	a.closing = true
	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
	}

	a.persist()
}

// handlePlayerData 返回玩家内存中的最新数据。
func (a *Processor) handlePlayerData(ctx node.Context) {
	res := &protocol.PlayerDataRes{Code: codes.InternalError.Code()}

	defer func() {
		if err := ctx.Response(res); err != nil {
			log.Errorf("response player data failed: %v", err)
		}
	}()

	res.Code = codes.OK.Code()
	res.Player = a.snapshot()
	res.Node = ctx.Proxy().GetID()
}

// handleAddExp 增加经验，经验达到阈值时自动升级。
func (a *Processor) handleAddExp(ctx node.Context) {
	req := &protocol.AddExpReq{}
	res := &protocol.AddExpRes{Code: codes.InternalError.Code()}

	defer func() {
		if err := ctx.Response(res); err != nil {
			log.Errorf("response add exp failed: %v", err)
		}
	}()

	if err := ctx.Parse(req); err != nil {
		log.Errorf("parse add exp request failed: %v", err)
		res.Code = codes.InvalidArgument.Code()
		return
	}
	if a.conflict {
		res.Message = "player data version conflict; reconnect required"
		return
	}
	if req.Delta <= 0 || req.Delta > maxDelta || a.data.Exp > math.MaxInt64-req.Delta || a.data.Level <= 0 || a.data.Level >= math.MaxInt32 || a.data.Version == math.MaxInt64 {
		res.Code = codes.InvalidArgument.Code()
		res.Message = "invalid delta or player value limit reached"
		return
	}

	a.gainExp(req.Delta)
	a.touch()

	log.Infof("player exp updated, uid: %d, delta: %d, level: %d, exp: %d",
		a.data.UID, req.Delta, a.data.Level, a.data.Exp)

	res.Code = codes.OK.Code()
	res.Message = "add exp ok"
	res.Player = a.snapshot()
	res.Node = ctx.Proxy().GetID()
}

// handleAddGold 增加金币。
func (a *Processor) handleAddGold(ctx node.Context) {
	req := &protocol.AddGoldReq{}
	res := &protocol.AddGoldRes{Code: codes.InternalError.Code()}

	defer func() {
		if err := ctx.Response(res); err != nil {
			log.Errorf("response add gold failed: %v", err)
		}
	}()

	if err := ctx.Parse(req); err != nil {
		log.Errorf("parse add gold request failed: %v", err)
		res.Code = codes.InvalidArgument.Code()
		return
	}
	if a.conflict {
		res.Message = "player data version conflict; reconnect required"
		return
	}
	if req.Delta <= 0 || req.Delta > maxDelta || a.data.Gold > math.MaxInt64-req.Delta || a.data.Version == math.MaxInt64 {
		res.Code = codes.InvalidArgument.Code()
		res.Message = "invalid delta or player value limit reached"
		return
	}

	a.data.Gold += req.Delta
	a.touch()

	log.Infof("player gold updated, uid: %d, delta: %d, gold: %d", a.data.UID, req.Delta, a.data.Gold)

	res.Code = codes.OK.Code()
	res.Message = "add gold ok"
	res.Player = a.snapshot()
	res.Node = ctx.Proxy().GetID()
}

// gainExp 增加经验并处理升级。
func (a *Processor) gainExp(delta int64) {
	a.data.Exp += delta

	for a.data.Level < math.MaxInt32 && a.data.Exp >= int64(a.data.Level)*expPerLevel {
		a.data.Exp -= int64(a.data.Level) * expPerLevel
		a.data.Level++
	}
}

// touch 标记脏数据，并在延迟窗口后合并写库。
func (a *Processor) touch() {
	a.data.Version++
	a.data.UpdatedAt = time.Now()
	a.dirty = true
	a.scheduleSave()
}

// scheduleSave 从首次变更开始计时；后续变更不推迟保存，失败后自动重试。
func (a *Processor) scheduleSave() {
	if a.timer != nil || a.closing || a.conflict {
		return
	}

	timer, err := a.schedule(saveDelay, a.persist)
	if err != nil {
		log.Warnf("schedule save player failed, uid: %d, err: %v", a.data.UID, err)
		return
	}

	a.timer = timer
}

// persist 把内存数据写回 MongoDB（在 Actor 执行域内调用，天然串行）。
func (a *Processor) persist() {
	a.timer = nil

	if !a.dirty || a.conflict {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()

	if err := a.store.Save(ctx, a.data, a.savedVersion); err != nil {
		log.Errorf("save player data failed, uid: %d, err: %v", a.data.UID, err)
		a.conflict = errors.Is(err, playerstore.ErrVersionConflict)
		a.scheduleSave()
		return
	}
	a.savedVersion = a.data.Version
	a.dirty = false

	log.Debugf("player data saved, uid: %d, level: %d, exp: %d, gold: %d",
		a.data.UID, a.data.Level, a.data.Exp, a.data.Gold)
}

// Snapshot 返回玩家数据的内存副本。
//
// 必须在 Actor 执行域内调用（例如通过 actor.Invoke），否则可能与消息处理并发。
func (a *Processor) Snapshot() *protocol.Player {
	return a.snapshot()
}

// snapshot 返回玩家数据的内存副本。
func (a *Processor) snapshot() *protocol.Player {
	return &protocol.Player{
		UID:      a.data.UID,
		Nickname: a.data.Nickname,
		Level:    a.data.Level,
		Exp:      a.data.Exp,
		Gold:     a.data.Gold,
	}
}
