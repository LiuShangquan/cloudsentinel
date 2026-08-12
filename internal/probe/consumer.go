package probe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type streamJob struct {
	id      string
	message ExecutionMessage
}

type Consumer struct {
	client                     *redis.Client
	stream, group, name        string
	readCount                  int64
	block, idle, claimInterval time.Duration
	workers, capacity          int
	processor                  *Processor
	observer                   Observer
	log                        *slog.Logger
}

func NewConsumer(client *redis.Client, stream, group, name string, readCount int64, block, idle, claimInterval time.Duration, workers, capacity int, processor *Processor, log *slog.Logger) *Consumer {
	return &Consumer{client: client, stream: stream, group: group, name: name, readCount: readCount, block: block, idle: idle, claimInterval: claimInterval, workers: workers, capacity: capacity, processor: processor, observer: processor.observer, log: log}
}

func (c *Consumer) Run(ctx context.Context) error {
	if err := c.client.XGroupCreateMkStream(ctx, c.stream, c.group, "0").Err(); err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("create Redis consumer group: %w", err)
	}
	jobs := make(chan streamJob, c.capacity)
	var workers sync.WaitGroup
	for index := 0; index < c.workers; index++ {
		workers.Add(1)
		go func() { defer workers.Done(); c.worker(ctx, jobs) }()
	}
	var claimant sync.WaitGroup
	claimant.Add(1)
	go func() { defer claimant.Done(); c.claimLoop(ctx, jobs) }()
	c.readLoop(ctx, jobs)
	claimant.Wait()
	close(jobs)
	workers.Wait()
	return nil
}

func (c *Consumer) readLoop(ctx context.Context, jobs chan<- streamJob) {
	for {
		if ctx.Err() != nil {
			return
		}
		streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: c.group, Consumer: c.name, Streams: []string{c.stream, ">"}, Count: c.readCount, Block: c.block}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || errors.Is(err, context.Canceled) {
				continue
			}
			c.log.Error("read Redis stream", "error", err)
			if !wait(ctx, time.Second) {
				return
			}
			continue
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				job, err := decode(message)
				if err != nil {
					c.log.Error("decode Redis stream message", "message_id", message.ID, "error", err)
					continue
				}
				if !enqueue(ctx, jobs, job) {
					return
				}
				if c.observer != nil {
					c.observer.SetQueueLength(len(jobs))
				}
			}
		}
	}
}

func (c *Consumer) claimLoop(ctx context.Context, jobs chan<- streamJob) {
	ticker := time.NewTicker(c.claimInterval)
	defer ticker.Stop()
	start := "0-0"
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			messages, next, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: c.stream, Group: c.group, Consumer: c.name, MinIdle: c.idle, Start: start, Count: c.readCount}).Result()
			if err != nil {
				if !errors.Is(err, context.Canceled) && !errors.Is(err, redis.Nil) {
					c.log.Error("claim pending probes", "error", err)
				}
				continue
			}
			start = next
			for _, message := range messages {
				job, err := decode(message)
				if err == nil {
					if !enqueue(ctx, jobs, job) {
						return
					}
					if c.observer != nil {
						c.observer.SetQueueLength(len(jobs))
					}
				}
			}
		}
	}
}

func (c *Consumer) worker(ctx context.Context, jobs <-chan streamJob) {
	for job := range jobs {
		if c.observer != nil {
			c.observer.SetQueueLength(len(jobs))
		}
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					c.log.Error("probe worker panic recovered", "panic", recovered)
				}
			}()
			err := c.processor.Process(ctx, job.message, c.name)
			if err != nil {
				if !errors.Is(err, ErrExecutionBusy) && !errors.Is(err, context.Canceled) {
					c.log.Error("process probe execution", "execution_id", job.message.ExecutionID, "error", err)
				}
				return
			}
			if err := c.client.XAck(ctx, c.stream, c.group, job.id).Err(); err != nil && !errors.Is(err, context.Canceled) {
				c.log.Error("ack probe execution", "execution_id", job.message.ExecutionID, "error", err)
			}
		}()
	}
}

func decode(message redis.XMessage) (streamJob, error) {
	executionID := fmt.Sprint(message.Values["execution_id"])
	taskID, err := strconv.ParseUint(fmt.Sprint(message.Values["task_id"]), 10, 64)
	if err != nil || executionID == "" {
		return streamJob{}, errors.New("invalid execution message")
	}
	scheduled, err := time.Parse(time.RFC3339Nano, fmt.Sprint(message.Values["scheduled_at"]))
	if err != nil {
		return streamJob{}, err
	}
	return streamJob{id: message.ID, message: ExecutionMessage{ExecutionID: executionID, TaskID: taskID, ScheduledAt: scheduled}}, nil
}

func enqueue(ctx context.Context, jobs chan<- streamJob, job streamJob) bool {
	select {
	case <-ctx.Done():
		return false
	case jobs <- job:
		return true
	}
}
func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
