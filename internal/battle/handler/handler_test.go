package handler

import (
	"errors"
	"testing"

	"github.com/dobyte/due/v2/cluster/node"
)

type baseNodeContext = node.Context

type failedLeaveContext struct {
	baseNodeContext
	uid      int64
	attempts int
}

func (c *failedLeaveContext) Parse(any) error    { return nil }
func (c *failedLeaveContext) UID() int64         { return c.uid }
func (c *failedLeaveContext) Response(any) error { return nil }
func (c *failedLeaveContext) UnbindNode(...int64) error {
	c.attempts++
	return errors.New("locator unavailable")
}

func TestLeaveRetainsRoomWhenUnbindFails(t *testing.T) {
	h := New()
	h.rooms[1] = &Room{ID: "room-1", UID: 1, Hits: 3}
	ctx := &failedLeaveContext{uid: 1}
	h.Leave(ctx)
	h.Leave(ctx)
	if ctx.attempts != 2 {
		t.Fatalf("unbind attempted %d times, want 2", ctx.attempts)
	}
	if room := h.rooms[1]; room == nil || room.Hits != 3 {
		t.Fatal("failed leave must retain room for retry")
	}
}
