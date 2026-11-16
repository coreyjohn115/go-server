package gamedata

import (
	"due-server/internal/common/gamedata/generated"
	"testing"
)

func TestExportedTables(t *testing.T) {
	tables, err := Load("../../../config/generated")
	if err != nil {
		t.Fatal(err)
	}
	if row := tables.TbPlayerLevel.Get(1); row == nil || row.Exp != 100 {
		t.Fatal("missing initial level")
	}
	if row := tables.TbItem.Get(1001); row == nil || row.MaxStack <= 0 {
		t.Fatal("missing example item")
	}
}

func TestMissingTables(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("missing tables must fail startup")
	}
}

func TestInventoryConfigValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Tables)
	}{
		{"missing reward", func(t *Tables) { t.TbTask.Get(110001).RewardItem = 999999 }},
		{"task cycle", func(t *Tables) { t.TbTask.Get(110002).NextTask = 110001 }},
		{"bad event", func(t *Tables) { t.TbTask.Get(110001).EventType = 5 }},
		{"negative target", func(t *Tables) { t.TbTask.Get(110001).Target = -1 }},
		{"missing rule", func(t *Tables) { delete(t.TbItemRule.GetDataMap(), 1002) }},
		{"bad initial", func(t *Tables) { t.TbInitItem.Get(1002).Count = 0 }},
		{"duplicate task", func(t *Tables) { t.TbTask.GetDataMap()[999] = &gamedata.Task{Id: 999} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tables, err := Load("../../../config/generated")
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(tables)
			if err = validateInventory(tables); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}
