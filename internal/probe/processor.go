package probe

import (
	"context"
	"errors"
	"time"
)

var ErrExecutionBusy = errors.New("execution owned by another worker")

type Processor struct {
	repo        *Repository
	probes      map[string]Probe
	idleTimeout time.Duration
	observer    Observer
}

type Observer interface {
	ActiveInc()
	ActiveDec()
	Observe(ExecutionWork, Result, time.Duration)
	SetQueueLength(int)
}

func NewProcessor(repo *Repository, httpProbe, tcpProbe Probe, idle time.Duration, observers ...Observer) *Processor {
	var observer Observer
	if len(observers) > 0 {
		observer = observers[0]
	}
	return &Processor{repo: repo, probes: map[string]Probe{"http": httpProbe, "tcp": tcpProbe}, idleTimeout: idle, observer: observer}
}
func (p *Processor) Process(ctx context.Context, message ExecutionMessage, consumer string) error {
	work, err := p.repo.GetWork(ctx, message.ExecutionID)
	if err != nil {
		return err
	}
	if work.Execution.Status == ExecutionSuccess || work.Execution.Status == ExecutionFailed {
		return nil
	}
	now := time.Now().UTC()
	acquired, err := p.repo.Acquire(ctx, message.ExecutionID, consumer, now, now.Add(-p.idleTimeout))
	if err != nil {
		return err
	}
	if !acquired {
		return ErrExecutionBusy
	}
	implementation, ok := p.probes[work.Execution.ProbeType]
	if !ok {
		return errors.New("unsupported probe type")
	}
	started := time.Now()
	if p.observer != nil {
		p.observer.ActiveInc()
		defer p.observer.ActiveDec()
	}
	result, attempts := ExecuteWithRetry(ctx, implementation, Target{Type: work.Execution.ProbeType, Address: work.Execution.TargetSnapshot}, time.Duration(work.Task.TimeoutMilliseconds)*time.Millisecond, work.Task.MaxRetries, time.Duration(work.Task.RetryBaseDelayMilliseconds)*time.Millisecond)
	finished := time.Now().UTC()
	if p.observer != nil {
		p.observer.Observe(work, result, time.Since(started))
	}
	return p.repo.MarkFinal(ctx, message.ExecutionID, consumer, finished, time.Since(started).Milliseconds(), result, attempts)
}
