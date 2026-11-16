// Package bootstrap 负责进程级公共初始化：业务配置中心、服务注册中心与用户定位器。
//
// 这里集中创建基础设施，是为了让 cmd/gate 与 cmd/node 保持相同的装配方式，
// 后续替换注册中心（consul / nacos / polaris）或定位器时只需要改这一处。
package bootstrap

import (
	"github.com/dobyte/due/locate/redis/v2"
	"github.com/dobyte/due/registry/etcd/v2"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/config/file"
	"github.com/dobyte/due/v2/locate"
	"github.com/dobyte/due/v2/registry"
)

// Init 初始化业务配置中心。
//
// etc 目录只放框架启动参数，业务配置统一放在 ./config 目录，
// 之后可以用 config.Get("app.xxx") 读取，并支持 Watch 热更新。
func Init() {
	config.SetConfigurator(config.NewConfigurator(
		config.WithSources(
			file.NewSource(file.WithPath("./config")),
		),
	))
}

// Registry 创建服务注册中心。
//
// 默认使用 etcd；换成 consul 只需改为 consul.NewRegistry()。
func Registry() registry.Registry {
	return etcd.NewRegistry()
}

// Locator 创建用户定位器。
//
// 网关与节点通过它记录、查询 UID 所在的网关与节点，目前由 redis 实现。
func Locator() locate.Locator {
	return redis.NewLocator()
}
