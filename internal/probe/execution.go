package probe

import (
	"context"
	"errors"
	"time"
)

type Target struct {
	Type    string
	Address string
}

type Result struct {
	Success        bool
	HTTPStatusCode int
	ErrorCode      string
	ErrorMessage   string
}

type Probe interface {
	Execute(context.Context, Target) (Result, error)
}

func ExecuteWithRetry(ctx context.Context, implementation Probe, target Target, timeout time.Duration, maxRetries int, baseDelay time.Duration) (Result, int) {
	var result Result
	for attempt := 0; attempt <= maxRetries; attempt++ {
		attemptContext, cancel := context.WithTimeout(ctx, timeout)
		current, err := implementation.Execute(attemptContext, target)
		cancel()
		result = current
		if err != nil && result.ErrorMessage == "" {
			result.ErrorMessage = err.Error()
		}
		if result.Success || !retryable(result, err) || attempt == maxRetries {
			return result, attempt + 1
		}
		delay := baseDelay * time.Duration(1<<attempt)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return Result{Success: false, ErrorCode: "network_error", ErrorMessage: ctx.Err().Error()}, attempt + 1
		case <-timer.C:
		}
	}
	return result, maxRetries + 1
}

func retryable(result Result, err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if result.HTTPStatusCode >= 400 && result.HTTPStatusCode < 500 {
		return false
	}
	if result.HTTPStatusCode >= 500 {
		return true
	}
	switch result.ErrorCode {
	case "dns_error", "connection_refused", "timeout", "tls_error", "network_error":
		return true
	default:
		return err != nil
	}
}
