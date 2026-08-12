package probe

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type Publisher interface {
	Publish(context.Context, ExecutionMessage) error
}
type RedisPublisher struct {
	client *redis.Client
	stream string
}

func NewPublisher(client *redis.Client, stream string) *RedisPublisher {
	return &RedisPublisher{client: client, stream: stream}
}
func (p *RedisPublisher) Publish(ctx context.Context, message ExecutionMessage) error {
	return p.client.XAdd(ctx, &redis.XAddArgs{Stream: p.stream, Values: map[string]any{"execution_id": message.ExecutionID, "task_id": strconv.FormatUint(message.TaskID, 10), "scheduled_at": message.ScheduledAt.UTC().Format(time.RFC3339Nano)}}).Err()
}
