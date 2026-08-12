package probe

import (
	"context"
	"log/slog"
	"time"
)

type Scheduler struct {
	repo      *Repository
	publisher Publisher
	interval  time.Duration
	batch     int
	log       *slog.Logger
}

func NewScheduler(repo *Repository, publisher Publisher, interval time.Duration, batch int, log *slog.Logger) *Scheduler {
	return &Scheduler{repo: repo, publisher: publisher, interval: interval, batch: batch, log: log}
}
func (s *Scheduler) Run(ctx context.Context) {
	s.cycle(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.cycle(ctx)
		}
	}
}
func (s *Scheduler) cycle(ctx context.Context) {
	messages, err := s.repo.ScheduleDue(ctx, s.batch, time.Now().UTC())
	if err != nil {
		s.log.Error("schedule due probes", "error", err)
		return
	}
	for _, message := range messages {
		if err := s.publisher.Publish(ctx, message); err != nil {
			s.log.Error("publish scheduled probe", "execution_id", message.ExecutionID, "error", err)
		}
	}
}

type RecoveryDispatcher struct {
	repo      *Repository
	publisher Publisher
	interval  time.Duration
	batch     int
	log       *slog.Logger
}

func NewRecoveryDispatcher(repo *Repository, publisher Publisher, interval time.Duration, batch int, log *slog.Logger) *RecoveryDispatcher {
	return &RecoveryDispatcher{repo: repo, publisher: publisher, interval: interval, batch: batch, log: log}
}
func (d *RecoveryDispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			messages, err := d.repo.QueuedBefore(ctx, time.Now().UTC().Add(-2*d.interval), d.batch)
			if err != nil {
				d.log.Error("scan queued probes", "error", err)
				continue
			}
			for _, message := range messages {
				if err := d.publisher.Publish(ctx, message); err != nil {
					d.log.Error("republish queued probe", "execution_id", message.ExecutionID, "error", err)
				}
			}
		}
	}
}
