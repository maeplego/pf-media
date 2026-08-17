package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

const streamKey = "media:jobs"

type JobMessage struct {
	JobID     string `json:"jobId"`
	FileID    string `json:"fileId"`
	ObjectKey string `json:"objectKey"`
	Bucket    string `json:"bucket"`
}

type Redis struct {
	rdb *redis.Client
}

func New(redisURL string) (*Redis, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	return &Redis{rdb: redis.NewClient(opt)}, nil
}

func (q *Redis) Ping(ctx context.Context) error {
	return q.rdb.Ping(ctx).Err()
}

func (q *Redis) Enqueue(ctx context.Context, msg JobMessage) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{"payload": string(b)},
	}).Err()
}

func (q *Redis) Read(ctx context.Context, group, consumer string, block time.Duration) (string, JobMessage, error) {
	res, err := q.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{streamKey, ">"},
		Count:    1,
		Block:    block,
	}).Result()
	if err != nil {
		return "", JobMessage{}, err
	}
	if len(res) == 0 || len(res[0].Messages) == 0 {
		return "", JobMessage{}, redis.Nil
	}
	m := res[0].Messages[0]
	raw, _ := m.Values["payload"].(string)
	var msg JobMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		return m.ID, JobMessage{}, err
	}
	return m.ID, msg, nil
}

func (q *Redis) EnsureGroup(ctx context.Context, group string) error {
	err := q.rdb.XGroupCreateMkStream(ctx, streamKey, group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

func (q *Redis) Ack(ctx context.Context, group, id string) error {
	return q.rdb.XAck(ctx, streamKey, group, id).Err()
}
