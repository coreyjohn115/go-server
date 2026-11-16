// Package handler 实现 robot 的协议检查与多连接测试流程。
package handler

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/cluster/client"
	"github.com/dobyte/due/v2/codes"

	"due-server/internal/common/protocol"
)

type Options struct {
	Addr    string
	UID     int64
	Count   int
	Token   string
	Timeout time.Duration
}

func (o Options) Validate() error {
	if o.Addr == "" || o.UID <= 0 || o.Count < 1 || o.Count > 1000 || o.Timeout <= 0 {
		return fmt.Errorf("addr must be nonempty, uid positive, count 1-1000 and timeout positive")
	}
	if o.UID > math.MaxInt64-int64(o.Count-1) {
		return fmt.Errorf("robot UID range overflows int64")
	}
	return nil
}

type reply struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Player  *protocol.Player `json:"player"`
	Node    string           `json:"node"`
	route   int32
	err     error
}

type mailboxKey struct{}

type Runner struct{ opts Options }

func New(opts Options) *Runner { return &Runner{opts: opts} }

// Register 必须在 client 启动之前调用。每个连接拥有独立的响应队列。
func (r *Runner) Register(proxy *client.Proxy) {
	for _, route := range []int32{protocol.RouteLogin, protocol.RoutePlayerData, protocol.RoutePing} {
		proxy.AddRouteHandler(route, func(ctx *client.Context) {
			res := reply{route: ctx.Route()}
			res.err = ctx.Parse(&res)
			deliver(ctx.Conn(), res)
		})
	}
	proxy.AddEventListener(cluster.Disconnect, func(conn *client.Conn) {
		deliver(conn, reply{err: fmt.Errorf("connection closed")})
	})
}

func deliver(conn *client.Conn, res reply) {
	box, ok := conn.GetAttr(mailboxKey{}).Value().(chan reply)
	if !ok {
		return
	}
	select {
	case box <- res:
	default:
	}
}

// Run 等待全部机器人完成，并返回失败数量。
func (r *Runner) Run(ctx context.Context, proxy *client.Proxy) int {
	var wg sync.WaitGroup
	var mu sync.Mutex
	failed := 0
	started := time.Now()
	for i := 0; i < r.opts.Count; i++ {
		uid := r.opts.UID + int64(i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := r.check(ctx, proxy, uid)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				fmt.Printf("robot FAIL uid=%d: %v\n", uid, err)
			} else {
				fmt.Printf("robot PASS uid=%d: login, playerData, ping\n", uid)
			}
		}()
	}
	wg.Wait()
	fmt.Printf("robot summary: total=%d passed=%d failed=%d elapsed=%s\n", r.opts.Count, r.opts.Count-failed, failed, time.Since(started).Round(time.Millisecond))
	return failed
}

func (r *Runner) check(parent context.Context, proxy *client.Proxy, uid int64) error {
	ctx, cancel := context.WithTimeout(parent, r.opts.Timeout)
	defer cancel()
	box := make(chan reply, 4)
	// Dial 本身使用 TCP 配置中的 dialTimeout；外层 deadline 还限制协议检查。
	conn, err := proxy.Dial(client.WithDialAddr(r.opts.Addr))
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()
	conn.SetAttr(mailboxKey{}, box)
	steps := []struct {
		route int32
		body  any
	}{
		{protocol.RouteLogin, &protocol.LoginReq{UID: uid, Token: r.opts.Token}},
		{protocol.RoutePlayerData, &protocol.PlayerDataReq{}},
		{protocol.RoutePing, &protocol.PingReq{Message: "robot"}},
	}
	for _, step := range steps {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := conn.Push(&cluster.Message{Route: step.route, Data: step.body}); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("route %d: %w", step.route, ctx.Err())
		case res := <-box:
			if res.err != nil {
				return res.err
			}
			if res.route != step.route || res.Code != codes.OK.Code() {
				return fmt.Errorf("expected route %d, got route=%d code=%d message=%s", step.route, res.route, res.Code, res.Message)
			}
			if step.route != protocol.RoutePing && (res.Player == nil || res.Player.UID != uid) {
				return fmt.Errorf("route %d: unexpected player UID", step.route)
			}
		}
	}
	return nil
}
