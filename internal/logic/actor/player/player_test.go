package playeractor

import (
	"testing"

	"due-server/internal/logic/store/playerstore"
)

func TestGainExp(t *testing.T) {
	tests := []struct {
		name       string
		startLevel int32
		startExp   int64
		delta      int64
		wantLevel  int32
		wantExp    int64
	}{
		{name: "no level up", startLevel: 1, startExp: 0, delta: 50, wantLevel: 1, wantExp: 50},
		{name: "exact level up", startLevel: 1, startExp: 0, delta: 100, wantLevel: 2, wantExp: 0},
		{name: "level up with remainder", startLevel: 1, startExp: 0, delta: 250, wantLevel: 2, wantExp: 150},
		{name: "multi level up", startLevel: 2, startExp: 0, delta: 300, wantLevel: 3, wantExp: 100},
		{name: "keep exp", startLevel: 2, startExp: 50, delta: 50, wantLevel: 2, wantExp: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Processor{data: &playerstore.Player{Level: tt.startLevel, Exp: tt.startExp}}

			a.gainExp(tt.delta)

			if a.data.Level != tt.wantLevel || a.data.Exp != tt.wantExp {
				t.Fatalf("gainExp(%d) from level=%d exp=%d = level=%d exp=%d, want level=%d exp=%d",
					tt.delta, tt.startLevel, tt.startExp,
					a.data.Level, a.data.Exp, tt.wantLevel, tt.wantExp)
			}
		})
	}
}

func TestSnapshot(t *testing.T) {
	a := &Processor{data: &playerstore.Player{
		UID:      1001,
		Nickname: "player-1001",
		Level:    3,
		Exp:      20,
		Gold:     500,
	}}

	snap := a.Snapshot()
	if snap.UID != 1001 || snap.Nickname != "player-1001" || snap.Level != 3 || snap.Exp != 20 || snap.Gold != 500 {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}

	// 快照应是副本，修改快照不影响 Actor 内部数据
	snap.Gold = 0
	if a.data.Gold != 500 {
		t.Fatalf("snapshot is not a copy, actor gold = %d", a.data.Gold)
	}
}
