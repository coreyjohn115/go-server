// Package app 负责把 backend HTTP 服务的路由装配到 due 组件上。
package app

import (
	dhttp "github.com/dobyte/due/component/http/v2"

	"due-server/internal/backend/handler"
	"due-server/internal/common/backendapi"
)

// RegisterBackend 注册 backend HTTP 服务的路由。
//
// backend 不接入 due 集群，只通过 HTTP 对外（以及向 logic）提供服务。
func RegisterBackend(proxy *dhttp.Proxy) {
	router := proxy.Router()

	router.Get(backendapi.PathHealth, handler.Health)
	router.Get(backendapi.PathServerInfo, handler.ServerInfo)
	router.Post(backendapi.PathVerifyToken, handler.VerifyToken)
	router.Get(backendapi.PathPlayer, handler.Player)
}
