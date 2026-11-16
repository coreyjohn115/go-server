package protocol

import (
	codec "github.com/dobyte/due/v2/encoding/proto"
	"testing"
)

func TestCodecRoundTrip(t *testing.T) {
	want := &LoginRes{Player: &Player{UID: 9007199254740993, Gold: 123}, Node: "logic-1"}
	data, err := codec.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	got := &LoginRes{}
	if err := codec.Unmarshal(data, got); err != nil {
		t.Fatal(err)
	}
	if got.Player.UID != want.Player.UID || got.Player.Gold != 123 || got.Node != want.Node {
		t.Fatalf("unexpected response: %v", got)
	}
}
