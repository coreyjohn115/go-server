// Package backendapi 定义 backend HTTP 后台服务的接口契约与调用客户端。
//
// backend 是独立部署的无状态 HTTP 服务（cmd/backend），不参与 due 集群，
// 只提供账号校验、运营查询等接口；logic 通过本包的 Client 调用它。
package backendapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dobyte/due/v2/codes"
)

// 接口路径
const (
	// PathHealth 健康检查
	PathHealth = "/healthz"
	// PathServerInfo 服务信息
	PathServerInfo = "/api/v1/server/info"
	// PathVerifyToken 登录令牌校验（logic 调用）
	PathVerifyToken = "/api/v1/auth/verify"
	// PathPlayer 玩家信息查询（运营后台使用）
	PathPlayer = "/api/v1/players/:uid"
)

// VerifyTokenReq 令牌校验请求
type VerifyTokenReq struct {
	UID   int64  `json:"uid"`
	Token string `json:"token"`
}

// VerifyTokenData 令牌校验结果
type VerifyTokenData struct {
	OK       bool   `json:"ok"`
	Nickname string `json:"nickname"`
}

// Player 玩家信息
type Player struct {
	UID      int64  `json:"uid"`
	Nickname string `json:"nickname"`
	Level    int    `json:"level"`
}

// ServerInfo 后台服务信息
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Time    string `json:"time"`
}

// Health 健康检查结果
type Health struct {
	Status string `json:"status"`
	Time   string `json:"time"`
}

// envelope 是 due http 组件统一响应结构
type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// Client 是 backend HTTP 服务的客户端。
//
// 内部使用短超时的 HTTP 客户端，避免 backend 故障拖垮调用方。
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient 创建 backend 客户端。
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
}

// VerifyToken 校验登录令牌。
func (c *Client) VerifyToken(ctx context.Context, uid int64, token string) (*VerifyTokenData, error) {
	data := &VerifyTokenData{}
	if err := c.do(ctx, http.MethodPost, PathVerifyToken, &VerifyTokenReq{UID: uid, Token: token}, data); err != nil {
		return nil, err
	}

	return data, nil
}

// Player 查询玩家信息。
func (c *Client) Player(ctx context.Context, uid int64) (*Player, error) {
	data := &Player{}
	path := strings.Replace(PathPlayer, ":uid", fmt.Sprintf("%d", uid), 1)
	if err := c.do(ctx, http.MethodGet, path, nil, data); err != nil {
		return nil, err
	}

	return data, nil
}

// do 发送请求并解析 due http 组件的统一响应。
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request failed: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request backend failed: %w", err)
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read backend response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("backend http status %d: %s", resp.StatusCode, strings.TrimSpace(string(content)))
	}

	env := &envelope{}
	if err = json.Unmarshal(content, env); err != nil {
		return fmt.Errorf("decode backend response failed: %w", err)
	}
	if env.Code != codes.OK.Code() {
		return fmt.Errorf("backend error: code=%d message=%s", env.Code, env.Message)
	}
	if out == nil || len(env.Data) == 0 {
		return nil
	}
	if err = json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("decode backend data failed: %w", err)
	}

	return nil
}
