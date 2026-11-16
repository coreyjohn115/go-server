package gamedata

import "testing"

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
