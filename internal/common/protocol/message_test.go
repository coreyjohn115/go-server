package protocol

import (
	codec "github.com/dobyte/due/v2/encoding/proto"
	"google.golang.org/protobuf/proto"
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

func TestInventoryCodecRoundTrip(t *testing.T) {
	want := &CommitTaskRes{Player: &Player{UID: 1,
		Items: []*ItemStack{{Id: 9007199254740993, ConfigId: 1002, Count: 3}},
		Tasks: []*TaskState{{Id: 110001, Progress: 1, Target: 1, Status: TaskStatus_TASK_STATUS_COMMITTED}},
	}}
	data, err := codec.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	got := &CommitTaskRes{}
	if err := codec.Unmarshal(data, got); err != nil {
		t.Fatal(err)
	}
	if !proto.Equal(want, got) {
		t.Fatalf("inventory protocol changed: %v", got)
	}
}
