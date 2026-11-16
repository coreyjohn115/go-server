// Command backend 启动 backend HTTP 后台服务。
//
// 该服务无状态、独立部署，不注册到 due 集群，也不依赖 etcd / redis / mongo；
// 提供登录校验与运营查询接口，logic 通过 HTTP 调用它完成登录校验。
package main

import (
	dhttp "github.com/dobyte/due/component/http/v2"
	"github.com/dobyte/due/v2"

	"due-server/internal/backend/app"
	"due-server/internal/common/bootstrap"
)

func main() {
	// 初始化业务配置中心（读取 app.token / app.name）
	bootstrap.Init()

	container := due.NewContainer()

	component := dhttp.NewServer()

	app.RegisterBackend(component.Proxy())

	container.Add(component)
	container.Serve()
}
