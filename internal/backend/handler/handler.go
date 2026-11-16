// Package handler 实现 backend HTTP 后台服务的接口。
//
// backend 是无状态服务：不保存业务状态、不注册到 due 集群，
// 只提供登录校验、运营查询等能力，可独立扩容、独立发布。
package handler

import (
	"fmt"
	"strconv"

	dhttp "github.com/dobyte/due/component/http/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xtime"

	"due-server/internal/common/backendapi"
)

// Health 健康检查。
func Health(ctx dhttp.Context) error {
	return ctx.Success(&backendapi.Health{
		Status: "ok",
		Time:   xtime.Now().Format(xtime.DateTime),
	})
}

// ServerInfo 返回后台服务信息。
func ServerInfo(ctx dhttp.Context) error {
	return ctx.Success(&backendapi.ServerInfo{
		Name:    config.Get("app.app.name", "due-server").String(),
		Version: due.Version,
		Time:    xtime.Now().Format(xtime.DateTime),
	})
}

// VerifyToken 校验登录令牌，供 logic 节点调用。
func VerifyToken(ctx dhttp.Context) error {
	req := &backendapi.VerifyTokenReq{}

	if err := ctx.Bind().JSON(req); err != nil {
		return ctx.Failure(codes.InvalidArgument)
	}
	if req.UID <= 0 {
		return ctx.Failure(codes.InvalidArgument)
	}

	data := &backendapi.VerifyTokenData{
		OK: req.Token == config.Get("app.app.token", "dev-token").String(),
	}
	if data.OK {
		data.Nickname = fmt.Sprintf("player-%d", req.UID)
	} else {
		log.Warnf("invalid token, uid: %d", req.UID)
	}

	return ctx.Success(data)
}

// Player 查询玩家信息（运营后台示例接口）。
func Player(ctx dhttp.Context) error {
	uid, err := strconv.ParseInt(ctx.Params("uid"), 10, 64)
	if err != nil || uid <= 0 {
		return ctx.Failure(codes.InvalidArgument)
	}

	return ctx.Success(&backendapi.Player{
		UID:      uid,
		Nickname: fmt.Sprintf("player-%d", uid),
		Level:    int(uid%10) + 1,
	})
}
