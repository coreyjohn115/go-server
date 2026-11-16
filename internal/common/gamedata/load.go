package gamedata

import (
	generated "due-server/internal/common/gamedata/generated"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Tables = generated.Tables

// Load validates a complete read-only table snapshot before publishing it to business handlers.
func Load(dir string) (*Tables, error) {
	tables, err := generated.NewTables(func(name string) ([]map[string]interface{}, error) {
		data, err := os.ReadFile(filepath.Join(dir, name+".json"))
		if err != nil {
			return nil, err
		}
		var rows []map[string]interface{}
		if err := json.Unmarshal(data, &rows); err != nil {
			return nil, fmt.Errorf("decode %s: %w", name, err)
		}
		return rows, nil
	})
	if err != nil {
		return nil, fmt.Errorf("load game tables: %w", err)
	}
	levels := tables.TbPlayerLevel.GetDataList()
	if len(levels) == 0 {
		return nil, fmt.Errorf("player level table is empty")
	}
	for i, row := range levels {
		if row.Level != int32(i+1) || row.Exp <= 0 {
			return nil, fmt.Errorf("invalid player level at row %d", i+1)
		}
	}
	items := tables.TbItem.GetDataList()
	if len(items) != len(tables.TbItem.GetDataMap()) {
		return nil, fmt.Errorf("duplicate item ID")
	}
	for _, row := range items {
		if row.Id <= 0 || row.Name == "" || row.MaxStack <= 0 {
			return nil, fmt.Errorf("invalid item %d", row.Id)
		}
	}
	return tables, nil
}
