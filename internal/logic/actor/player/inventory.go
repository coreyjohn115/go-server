package playeractor

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"due-server/internal/common/protocol"
	"due-server/internal/logic/store/playerstore"
	"github.com/dobyte/due/v2/cluster/node"
	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/log"
)

const (
	taskAccepted  int32 = 1
	taskFinished  int32 = 2
	taskCommitted int32 = 3
	eventUse      int32 = 2
	eventAdd      int32 = 3
	eventConsume  int32 = 4
	maxStacks           = 10000
)

// transact stages all effects, events and rewards on a private copy. A failed
// reward or use leaves inventory, task progress and version completely unchanged.
func (a *Processor) transact(fn func(*Processor) error) error {
	if a.conflict || a.closing || a.data.Version == math.MaxInt64 {
		return errors.New("player is unavailable for asset changes")
	}
	if a.tables == nil {
		return errors.New("game tables unavailable")
	}
	copyData := *a.data
	copyData.Items = slices.Clone(a.data.Items)
	copyData.Tasks = slices.Clone(a.data.Tasks)
	copyData.InitItemIDs = slices.Clone(a.data.InitItemIDs)
	staged := &Processor{data: &copyData, tables: a.tables}
	if err := fn(staged); err != nil {
		return err
	}
	if err := staged.autoCommitTasks(); err != nil {
		return err
	}
	*a.data = copyData
	a.touch()
	return nil
}

func (a *Processor) initializeInventory() error {
	if a.tables == nil {
		return nil
	}
	// Avoid a write on every login when all configured initial entries exist.
	needed := false
	for _, cfg := range a.tables.TbTask.GetDataList() {
		if cfg.Initial && a.findTask(cfg.Id) == nil {
			needed = true
		}
	}
	for _, cfg := range a.tables.TbInitItem.GetDataList() {
		if !slices.Contains(a.data.InitItemIDs, cfg.Id) {
			needed = true
		}
	}
	if !needed {
		return nil
	}
	return a.transact(func(p *Processor) error {
		// Accept tasks before initial grants so acquisition events are counted.
		for _, cfg := range p.tables.TbTask.GetDataList() {
			if cfg.Initial {
				p.acceptTask(cfg.Id)
			}
		}
		for _, cfg := range p.tables.TbInitItem.GetDataList() {
			if slices.Contains(p.data.InitItemIDs, cfg.Id) {
				continue
			}
			if err := p.addItem(cfg.Id, cfg.Count); err != nil {
				return err
			}
			p.data.InitItemIDs = append(p.data.InitItemIDs, cfg.Id)
		}
		return nil
	})
}

// grantItem is a server-side reward primitive; clients cannot grant assets.
func (a *Processor) grantItem(id int32, count int64) error {
	cfg := a.tables.TbItem.Get(id)
	rule := a.tables.TbItemRule.Get(id)
	if cfg == nil || rule == nil || count <= 0 || count > maxDelta {
		return errors.New("invalid item grant")
	}
	if rule.Mode == 1 {
		if rule.Value <= 0 || count > math.MaxInt64/rule.Value || a.data.Gold > math.MaxInt64-count*rule.Value {
			return errors.New("gold limit reached")
		}
		a.data.Gold += count * rule.Value
	} else {
		for i := range a.data.Items {
			item := &a.data.Items[i]
			if item.ConfigID != id || item.Count >= int64(cfg.MaxStack) {
				continue
			}
			n := min(count, int64(cfg.MaxStack)-item.Count)
			item.Count += n
			count -= n
			if count == 0 {
				break
			}
		}
		remaining := count
		for remaining > 0 {
			if len(a.data.Items) >= maxStacks || a.data.NextItemID == math.MaxInt64 {
				return errors.New("inventory capacity reached")
			}
			a.data.NextItemID++
			n := min(remaining, int64(cfg.MaxStack))
			a.data.Items = append(a.data.Items, playerstore.Item{ID: a.data.NextItemID, ConfigID: id, Count: n})
			remaining -= n
		}
	}
	return nil
}

func (a *Processor) addItem(id int32, count int64) error {
	if err := a.grantItem(id, count); err != nil {
		return err
	}
	a.taskEvent(eventAdd, id, count)
	return nil
}

func (a *Processor) changeItem(id, count int64, use bool) error {
	return a.transact(func(p *Processor) error {
		if count <= 0 || count > maxDelta {
			return errors.New("invalid item count")
		}
		idx := slices.IndexFunc(p.data.Items, func(v playerstore.Item) bool { return v.ID == id })
		if idx < 0 || p.data.Items[idx].Count < count {
			return errors.New("item not found or not enough")
		}
		item := p.data.Items[idx]
		rule := p.tables.TbItemRule.Get(item.ConfigID)
		if rule == nil {
			return errors.New("item configuration missing")
		}
		// Consume first on the staged copy, freeing capacity for box contents.
		p.data.Items[idx].Count -= count
		if p.data.Items[idx].Count == 0 {
			p.data.Items = slices.Delete(p.data.Items, idx, idx+1)
		}
		if use {
			switch rule.Mode {
			case 1, 2:
				if rule.Value <= 0 || count > math.MaxInt64/rule.Value {
					return errors.New("item effect overflow")
				}
				delta := count * rule.Value
				if rule.Mode == 1 {
					if p.data.Gold > math.MaxInt64-delta {
						return errors.New("gold limit reached")
					}
					p.data.Gold += delta
				} else {
					if p.data.Exp > math.MaxInt64-delta || p.data.Level <= 0 || p.data.Level == math.MaxInt32 {
						return errors.New("experience limit reached")
					}
					p.gainExp(delta)
				}
			case 3: // Server validates consumption; the game client applies its board effect.
			case 4:
				if rule.RewardCount <= 0 || count > maxDelta/rule.RewardCount {
					return errors.New("box reward limit reached")
				}
				if err := p.addItem(rule.RewardItem, count*rule.RewardCount); err != nil {
					return err
				}
			default:
				return errors.New("item cannot be used")
			}
			p.taskEvent(eventUse, item.ConfigID, count)
		}
		p.taskEvent(eventConsume, item.ConfigID, count)
		return nil
	})
}

func (a *Processor) findTask(id int32) *playerstore.Task {
	for i := range a.data.Tasks {
		if a.data.Tasks[i].ID == id {
			return &a.data.Tasks[i]
		}
	}
	return nil
}

func (a *Processor) acceptTask(id int32) {
	if a.findTask(id) != nil {
		return
	}
	cfg := a.tables.TbTask.Get(id)
	if cfg == nil {
		return
	}
	t := playerstore.Task{ID: id, Status: taskAccepted}
	if cfg.EventType == 1 {
		t.Progress, t.Status = cfg.Target, taskFinished
	}
	a.data.Tasks = append(a.data.Tasks, t)
}

func (a *Processor) taskEvent(event, itemID int32, count int64) {
	if count <= 0 {
		return
	}
	for i := range a.data.Tasks {
		t := &a.data.Tasks[i]
		cfg := a.tables.TbTask.Get(t.ID)
		if cfg == nil || t.Status != taskAccepted || cfg.EventType != event || (cfg.ItemId != 0 && cfg.ItemId != itemID) {
			continue
		}
		t.Progress += min(count, max(int64(0), cfg.Target-t.Progress))
		if t.Progress >= cfg.Target {
			t.Status = taskFinished
		}
	}
}

func (a *Processor) commitTask(id int32) error {
	return a.transact(func(p *Processor) error { return p.commitTaskReward(id) })
}

func (a *Processor) commitTaskReward(id int32) error {
	t := a.findTask(id)
	cfg := a.tables.TbTask.Get(id)
	if t == nil || cfg == nil {
		return errors.New("task not accepted")
	}
	if t.Status == taskCommitted {
		return nil
	} // Retry is successful without a second reward.
	if t.Status != taskFinished {
		return errors.New("task not finished")
	}
	t.Status = taskCommitted
	if cfg.NextTask != 0 {
		a.acceptTask(cfg.NextTask)
	}
	if cfg.RewardItem != 0 {
		if err := a.addItem(cfg.RewardItem, cfg.RewardCount); err != nil {
			return err
		}
	}
	return nil
}

func (a *Processor) autoCommitTasks() error {
	// Each configured task is rewarded at most once, bounding event cascades.
	for pass := 0; pass <= len(a.tables.TbTask.GetDataList()); pass++ {
		changed := false
		for i := 0; i < len(a.data.Tasks); i++ {
			t := a.data.Tasks[i]
			cfg := a.tables.TbTask.Get(t.ID)
			if cfg != nil && cfg.AutoCommit && t.Status == taskFinished {
				if err := a.commitTaskReward(t.ID); err != nil {
					return err
				}
				changed = true
			}
		}
		if !changed {
			return nil
		}
	}
	return fmt.Errorf("task reward cascade exceeded configuration size")
}

func (a *Processor) itemSnapshot() []*protocol.ItemStack {
	list := make([]*protocol.ItemStack, 0, len(a.data.Items))
	for _, v := range a.data.Items {
		list = append(list, &protocol.ItemStack{Id: v.ID, ConfigId: v.ConfigID, Count: v.Count})
	}
	return list
}
func (a *Processor) taskSnapshot() []*protocol.TaskState {
	list := make([]*protocol.TaskState, 0, len(a.data.Tasks))
	for _, v := range a.data.Tasks {
		var target int64
		if a.tables != nil {
			if cfg := a.tables.TbTask.Get(v.ID); cfg != nil {
				target = cfg.Target
			}
		}
		list = append(list, &protocol.TaskState{Id: v.ID, Progress: v.Progress, Target: target, Status: protocol.TaskStatus(v.Status)})
	}
	return list
}

func respondInventory(ctx node.Context, res any) {
	if err := ctx.Response(res); err != nil {
		log.Errorf("response inventory failed: %v", err)
	}
}
func (a *Processor) handleItemList(ctx node.Context) {
	res := &protocol.ItemListRes{Code: int32(codes.OK.Code()), Items: a.itemSnapshot()}
	if ctx.Parse(&protocol.ItemListReq{}) != nil {
		res.Code = int32(codes.InvalidArgument.Code())
		res.Items = nil
	}
	respondInventory(ctx, res)
}
func (a *Processor) handleTaskList(ctx node.Context) {
	res := &protocol.TaskListRes{Code: int32(codes.OK.Code()), Tasks: a.taskSnapshot()}
	if ctx.Parse(&protocol.TaskListReq{}) != nil {
		res.Code = int32(codes.InvalidArgument.Code())
		res.Tasks = nil
	}
	respondInventory(ctx, res)
}
func (a *Processor) handleUseItem(ctx node.Context) {
	req := &protocol.UseItemReq{}
	err := ctx.Parse(req)
	if err == nil {
		err = a.changeItem(req.Id, req.Count, true)
	}
	a.respondItemChange(ctx, err)
}
func (a *Processor) handleDeleteItem(ctx node.Context) {
	req := &protocol.DeleteItemReq{}
	err := ctx.Parse(req)
	if err == nil {
		err = a.changeItem(req.Id, req.Count, false)
	}
	a.respondItemChange(ctx, err)
}
func (a *Processor) respondItemChange(ctx node.Context, err error) {
	res := &protocol.ItemChangeRes{Code: int32(codes.OK.Code())}
	if err != nil {
		res.Code = int32(codes.InvalidArgument.Code())
		res.Message = err.Error()
	} else {
		res.Player = a.snapshot()
	}
	respondInventory(ctx, res)
}
func (a *Processor) handleCommitTask(ctx node.Context) {
	req := &protocol.CommitTaskReq{}
	res := &protocol.CommitTaskRes{Code: int32(codes.OK.Code())}
	err := ctx.Parse(req)
	if err == nil {
		err = a.commitTask(req.Id)
	}
	if err != nil {
		res.Code = int32(codes.InvalidArgument.Code())
		res.Message = err.Error()
	} else {
		res.Player = a.snapshot()
	}
	respondInventory(ctx, res)
}
