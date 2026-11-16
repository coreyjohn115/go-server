package protocol

// Player 是下发给客户端的玩家数据快照。
type Player struct {
	UID      int64  `json:"uid"`
	Nickname string `json:"nickname"`
	Level    int32  `json:"level"`
	Exp      int64  `json:"exp"`
	Gold     int64  `json:"gold"`
}

// LoginReq 登录请求
type LoginReq struct {
	UID   int64  `json:"uid"`
	Token string `json:"token"`
}

// LoginRes 登录响应（带数据库中的玩家数据）
type LoginRes struct {
	Code    int     `json:"code"`
	Message string  `json:"message"`
	Player  *Player `json:"player"`
	// Node 处理本次请求的 logic 实例 ID，便于观察有状态路由的粘性
	Node string `json:"node"`
}

// PingReq 连通性测试请求
type PingReq struct {
	Message string `json:"message"`
}

// PingRes 连通性测试响应
type PingRes struct {
	Code       int    `json:"code"`
	Message    string `json:"message"`
	ServerTime string `json:"serverTime"`
	Node       string `json:"node"`
}

// PlayerDataReq 查询玩家数据请求（无请求参数）
type PlayerDataReq struct{}

// PlayerDataRes 查询玩家数据响应
type PlayerDataRes struct {
	Code    int     `json:"code"`
	Message string  `json:"message"`
	Player  *Player `json:"player"`
	Node    string  `json:"node"`
}

// AddExpReq 增加经验请求
type AddExpReq struct {
	Delta int64 `json:"delta"`
}

// AddExpRes 增加经验响应
type AddExpRes struct {
	Code    int     `json:"code"`
	Message string  `json:"message"`
	Player  *Player `json:"player"`
	Node    string  `json:"node"`
}

// AddGoldReq 增加金币请求
type AddGoldReq struct {
	Delta int64 `json:"delta"`
}

// AddGoldRes 增加金币响应
type AddGoldRes struct {
	Code    int     `json:"code"`
	Message string  `json:"message"`
	Player  *Player `json:"player"`
	Node    string  `json:"node"`
}

// EnterBattleReq 进入战斗请求
type EnterBattleReq struct {
	// Mode 战斗模式，示例值：1v1
	Mode string `json:"mode"`
}

// EnterBattleRes 进入战斗响应
type EnterBattleRes struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	RoomID  string `json:"roomId"`
	UID     int64  `json:"uid"`
	Node    string `json:"node"`
}

// AttackReq 攻击请求
type AttackReq struct {
	Skill string `json:"skill"`
}

// AttackRes 攻击响应
type AttackRes struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	RoomID  string `json:"roomId"`
	Hits    int    `json:"hits"`
	Node    string `json:"node"`
}

// LeaveBattleReq 离开战斗请求
type LeaveBattleReq struct{}

// LeaveBattleRes 离开战斗响应
type LeaveBattleRes struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	RoomID  string `json:"roomId"`
	Node    string `json:"node"`
}

// ErrorRes 是框架层错误（如 Actor 未绑定）的通用响应
type ErrorRes struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
