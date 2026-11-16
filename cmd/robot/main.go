// Command robot 登录网关并检查玩家数据与 ping 链路。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/cluster/client"
	"github.com/dobyte/due/v2/config"

	"due-server/internal/common/bootstrap"
	"due-server/internal/robot/app"
	"due-server/internal/robot/handler"
)

func main() {
	bootstrap.Init()
	var opts handler.Options
	flag.StringVar(&opts.Addr, "addr", "127.0.0.1:3553", "gate TCP address")
	flag.Int64Var(&opts.UID, "uid", 1001, "first robot UID (each robot increments by one)")
	flag.IntVar(&opts.Count, "count", 1, "number of robots (1-1000)")
	flag.StringVar(&opts.Token, "token", config.Get("app.app.token", "dev-token").String(), "login token")
	flag.DurationVar(&opts.Timeout, "timeout", 10*time.Second, "timeout per robot, including dial")
	flag.Parse()
	if err := opts.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	runner := handler.New(opts)
	component := client.NewClient(client.WithName("robot"), client.WithClient(tcp.NewClient(
		tcp.WithClientDialTimeout(opts.Timeout),
		tcp.WithClientWriteTimeout(opts.Timeout),
	)))
	app.RegisterRobot(component.Proxy(), runner)
	failed := 0
	component.Proxy().AddHookListener(cluster.Start, func(proxy *client.Proxy) {
		failed = runner.Run(ctx, proxy)
	})
	container := due.NewContainer()
	container.Add(component)
	// Start hook 完成全部检查后进入正常关闭流程。
	container.Serve(true)
	if failed != 0 {
		os.Exit(1)
	}
}
