package playeractor

import (
	"context"
	"math"
	"reflect"
	"testing"
	"time"

	"due-server/internal/common/gamedata"
	"due-server/internal/logic/store/playerstore"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func inventoryPlayer(t *testing.T) *Processor {
	t.Helper()
	tables, err := gamedata.Load("../../../../config/generated")
	if err != nil {
		t.Fatal(err)
	}
	p := &Processor{data: &playerstore.Player{UID: 1, Level: 1, Version: 1}, tables: tables, savedVersion: 1, store: &fakeStore{}}
	p.schedule = func(time.Duration, func()) (saveTimer, error) { return &fakeTimer{}, nil }
	if err = p.initializeInventory(); err != nil {
		t.Fatal(err)
	}
	return p
}

func itemID(p *Processor, config int32) int64 {
	for _, v := range p.data.Items {
		if v.ConfigID == config {
			return v.ID
		}
	}
	return 0
}
func itemCount(p *Processor, config int32) int64 {
	var n int64
	for _, v := range p.data.Items {
		if v.ConfigID == config {
			n += v.Count
		}
	}
	return n
}

func TestInventoryTaskLifecycle(t *testing.T) {
	p := inventoryPlayer(t)
	if p.findTask(100001).Status != taskFinished || p.findTask(110001).Progress != 0 {
		t.Fatal("initial item events incorrect")
	}
	if err := p.changeItem(itemID(p, 1002), 1, true); err != nil {
		t.Fatal(err)
	}
	if p.data.Exp != 50 || itemCount(p, 1002) != 2 || p.findTask(110001).Status != taskFinished || p.findTask(100002).Progress != 1 {
		t.Fatal("use did not apply effect and task events")
	}
	if err := p.commitTask(110001); err != nil {
		t.Fatal(err)
	}
	if p.data.Gold != 100 || p.findTask(110002) == nil {
		t.Fatal("reward/successor missing")
	}
	if err := p.commitTask(110001); err != nil || p.data.Gold != 100 {
		t.Fatal("duplicate reward")
	}
	if err := p.changeItem(itemID(p, 1003), 1, true); err != nil {
		t.Fatal(err)
	}
	if itemCount(p, 1002) != 4 || itemCount(p, 1003) != 0 {
		t.Fatal("box reward missing")
	}
	if err := p.changeItem(itemID(p, 1002), 3, true); err != nil {
		t.Fatal(err)
	}
	if p.findTask(110002).Status != taskFinished || p.data.Level != 2 || p.data.Exp != 100 {
		t.Fatal("successor progress/level incorrect")
	}
	if err := p.commitTask(110002); err != nil {
		t.Fatal(err)
	}
	if itemCount(p, 1003) != 1 {
		t.Fatal("successor reward missing")
	}
	before := p.snapshot()
	if err := p.initializeInventory(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, p.snapshot()) {
		t.Fatal("relogin repeated initial grants")
	}
	beforeProgress := p.findTask(100002).Progress
	if err := p.changeItem(itemID(p, 120001), 1, false); err != nil {
		t.Fatal(err)
	}
	if p.findTask(100002).Progress < beforeProgress {
		t.Fatal("deletion lost consume progress")
	}
}

func TestInventoryRollbackAndLimits(t *testing.T) {
	p := inventoryPlayer(t)
	for _, op := range []func() error{
		func() error { return p.changeItem(itemID(p, 1002), 0, true) },
		func() error { return p.changeItem(itemID(p, 1002), -1, true) },
		func() error { return p.changeItem(itemID(p, 1002), 4, true) },
		func() error { return p.changeItem(999, 1, false) },
		func() error { return p.commitTask(110001) },
		func() error { return p.commitTask(999) },
	} {
		before, _ := bson.Marshal(p.data)
		if err := op(); err == nil {
			t.Fatal("invalid operation accepted")
		}
		after, _ := bson.Marshal(p.data)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("failed operation changed persistent state")
		}
	}
	p.data.Gold = math.MaxInt64
	before, _ := bson.Marshal(p.data)
	if err := p.commitTask(100002); err == nil {
		t.Fatal("unfinished task accepted")
	}
	// Welcome task is finished. Its grant must roll back when stack IDs exhaust.
	p.data.NextItemID = math.MaxInt64
	p.data.Items = nil
	before, _ = bson.Marshal(p.data)
	if err := p.commitTask(100003); err == nil {
		t.Fatal("overflow accepted")
	}
	after, _ := bson.Marshal(p.data)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed reward marked task committed")
	}
	p.conflict = true
	if err := p.changeItem(1, 1, true); err == nil {
		t.Fatal("conflicted actor changed assets")
	}
}

func TestStacksEventsAndAutoCommit(t *testing.T) {
	p := inventoryPlayer(t)
	p.tables.TbTask.Get(100002).AutoCommit = true
	if err := p.transact(func(s *Processor) error { return s.addItem(1002, 200) }); err != nil {
		t.Fatal(err)
	}
	if itemCount(p, 1002) != 203 {
		t.Fatal("stack split lost items")
	}
	ids := map[int64]bool{}
	for _, v := range p.data.Items {
		if ids[v.ID] || v.Count > int64(p.tables.TbItem.Get(v.ConfigID).MaxStack) {
			t.Fatal("invalid stack")
		}
		ids[v.ID] = true
	}
	if err := p.changeItem(itemID(p, 120001), 3, true); err != nil {
		t.Fatal(err)
	}
	if p.findTask(100002).Status != taskCommitted || p.data.Gold != 50 {
		t.Fatal("auto reward missing")
	}
}

type roundTripStore struct{ data []byte }

func (s *roundTripStore) Save(_ context.Context, p *playerstore.Player, _ int64) error {
	var err error
	s.data, err = bson.Marshal(p)
	return err
}
func TestInventoryPersistenceRoundTrip(t *testing.T) {
	p := inventoryPlayer(t)
	if err := p.changeItem(itemID(p, 1002), 1, true); err != nil {
		t.Fatal(err)
	}
	if err := p.commitTask(110001); err != nil {
		t.Fatal(err)
	}
	s := &roundTripStore{}
	p.store = s
	p.persist()
	if p.dirty || len(s.data) == 0 {
		t.Fatal("assets not saved")
	}
	var restored playerstore.Player
	if err := bson.Unmarshal(s.data, &restored); err != nil {
		t.Fatal(err)
	}
	p.data = &restored
	before := p.snapshot()
	if err := p.initializeInventory(); err != nil {
		t.Fatal(err)
	}
	if err := p.commitTask(110001); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, p.snapshot()) {
		t.Fatal("restart reissued assets")
	}
	// A previously deployed player document has no new fields and migrates once.
	p.data = &playerstore.Player{UID: 2, Level: 1, Version: 10}
	if err := p.initializeInventory(); err != nil {
		t.Fatal(err)
	}
	if len(p.data.InitItemIDs) != 3 || itemCount(p, 1002) != 3 {
		t.Fatal("old player migration failed")
	}
}
