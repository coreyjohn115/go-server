// Package playerstore 封装玩家数据的 MongoDB 读写。
//
// 玩家文档以 UID 作为 _id，天然保证一个玩家一条记录；
// version 字段由 Actor 在数据变更时递增，保存时检查上次成功保存的版本。
package playerstore

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ErrVersionConflict 表示文档已被其他写入者修改或删除，禁止覆盖。
var ErrVersionConflict = errors.New("player version conflict")

// Player 是玩家数据文档。
type Player struct {
	UID       int64     `bson:"_id"`
	Nickname  string    `bson:"nickname"`
	Level     int32     `bson:"level"`
	Exp       int64     `bson:"exp"`
	Gold      int64     `bson:"gold"`
	Version   int64     `bson:"version"`
	CreatedAt time.Time `bson:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt"`
}

// Store 是玩家数据存储。
type Store struct {
	client *mongo.Client
	coll   *mongo.Collection
}

// New 连接 MongoDB 并返回玩家数据存储。
func New(ctx context.Context, uri, database, collection string) (*Store, error) {
	client, err := mongo.Connect(options.Client().
		ApplyURI(uri).
		SetConnectTimeout(5 * time.Second).
		SetServerSelectionTimeout(5 * time.Second))
	if err != nil {
		return nil, err
	}

	if err = client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, err
	}

	return &Store{
		client: client,
		coll:   client.Database(database).Collection(collection),
	}, nil
}

// Close 关闭连接。
func (s *Store) Close(ctx context.Context) error {
	return s.client.Disconnect(ctx)
}

// LoadOrCreate 读取玩家数据；不存在时按初始值创建。
func (s *Store) LoadOrCreate(ctx context.Context, uid int64, nickname string) (*Player, error) {
	player := &Player{}

	err := s.coll.FindOne(ctx, bson.M{"_id": uid}).Decode(player)
	if err == nil {
		return player, nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	now := time.Now()
	player = &Player{
		UID:       uid,
		Nickname:  nickname,
		Level:     1,
		Exp:       0,
		Gold:      0,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if _, err = s.coll.InsertOne(ctx, player); err != nil {
		// 并发登录时可能已被其他实例创建，回读一次即可
		if !mongo.IsDuplicateKeyError(err) {
			return nil, err
		}
		if err = s.coll.FindOne(ctx, bson.M{"_id": uid}).Decode(player); err != nil {
			return nil, err
		}
	}

	return player, nil
}

// Save 仅更新版本与上次加载/保存版本一致的文档，防止旧 Actor 覆盖新数据。
func (s *Store) Save(ctx context.Context, player *Player, expectedVersion int64) error {
	result, err := s.coll.UpdateOne(ctx, bson.M{"_id": player.UID, "version": expectedVersion}, bson.M{
		"$set": bson.M{
			"nickname":  player.Nickname,
			"level":     player.Level,
			"exp":       player.Exp,
			"gold":      player.Gold,
			"version":   player.Version,
			"updatedAt": player.UpdatedAt,
		},
	})

	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrVersionConflict
	}
	return nil
}
