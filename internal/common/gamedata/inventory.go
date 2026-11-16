package gamedata

import "fmt"

func validateInventory(t *Tables) error {
	if len(t.TbItemRule.GetDataList()) != len(t.TbItemRule.GetDataMap()) || len(t.TbInitItem.GetDataList()) != len(t.TbInitItem.GetDataMap()) || len(t.TbTask.GetDataList()) != len(t.TbTask.GetDataMap()) {
		return fmt.Errorf("duplicate inventory/task config ID")
	}
	for _, item := range t.TbItem.GetDataList() {
		if t.TbItemRule.Get(item.Id) == nil {
			return fmt.Errorf("item %d has no rule", item.Id)
		}
	}
	for _, r := range t.TbItemRule.GetDataList() {
		if t.TbItem.Get(r.Id) == nil || r.Mode < 0 || r.Mode > 4 || r.Value < 0 || r.RewardCount < 0 {
			return fmt.Errorf("invalid item rule %d", r.Id)
		}
		if (r.Mode == 1 || r.Mode == 2) && r.Value <= 0 {
			return fmt.Errorf("invalid effect value %d", r.Id)
		}
		if r.Mode == 4 && (t.TbItem.Get(r.RewardItem) == nil || r.RewardCount <= 0 || r.RewardCount > 1_000_000) {
			return fmt.Errorf("invalid box reward %d", r.Id)
		}
	}
	for _, r := range t.TbInitItem.GetDataList() {
		if t.TbItem.Get(r.Id) == nil || r.Count <= 0 || r.Count > 1_000_000 {
			return fmt.Errorf("invalid initial item %d", r.Id)
		}
	}
	for _, r := range t.TbTask.GetDataList() {
		if r.Id <= 0 || r.Name == "" || (r.TaskType != 10 && r.TaskType != 11) || r.EventType < 1 || r.EventType > 4 || r.Target <= 0 || (r.ItemId != 0 && t.TbItem.Get(r.ItemId) == nil) {
			return fmt.Errorf("invalid task %d", r.Id)
		}
		if (r.RewardItem == 0 && r.RewardCount != 0) || (r.RewardItem != 0 && (t.TbItem.Get(r.RewardItem) == nil || r.RewardCount <= 0 || r.RewardCount > 1_000_000)) {
			return fmt.Errorf("invalid task reward %d", r.Id)
		}
		seen := map[int32]bool{r.Id: true}
		for next := r.NextTask; next != 0; {
			cfg := t.TbTask.Get(next)
			if cfg == nil || seen[next] {
				return fmt.Errorf("missing or cyclic task successor %d", next)
			}
			seen[next] = true
			next = cfg.NextTask
		}
	}
	return nil
}
