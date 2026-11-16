// Package gate 负责构建网关组件。
//
// 网关只维护客户端连接与消息转发，不承载业务逻辑。
// 一个 gate 进程只挂一种网络协议；需要同时支持 tcp / ws / kcp / quic 时，
// 按协议拆分多个 gate 进程（同一个 gate 服务组）即可。
package gate

import (
	"github.com/dobyte/due/network/tcp/v2"
	duegate "github.com/dobyte/due/v2/cluster/gate"

	"due-server/internal/common/bootstrap"
)

// New 创建网关组件。
func New() *duegate.Gate {
	return duegate.NewGate(
		// 对外网络：tcp / kcp / ws / quic，按业务场景替换
		duegate.WithServer(tcp.NewServer()),
		// 用户定位：记录 UID 当前连接在哪个网关
		duegate.WithLocator(bootstrap.Locator()),
		// 服务发现：发现节点实例及其路由
		duegate.WithRegistry(bootstrap.Registry()),
	)
}
