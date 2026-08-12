package probe

import "time"

const (
	TaskActive       = "active"
	TaskDisabled     = "disabled"
	ExecutionQueued  = "queued"
	ExecutionRunning = "running"
	ExecutionSuccess = "succeeded"
	ExecutionFailed  = "failed"
)

type Task struct {
	ID                         uint64     `gorm:"primaryKey" json:"id"`
	ServiceID                  uint64     `json:"service_id"`
	Name                       string     `gorm:"size:100;not null" json:"name"`
	IntervalSeconds            int        `json:"interval_seconds"`
	TimeoutMilliseconds        int        `json:"timeout_milliseconds"`
	MaxRetries                 int        `json:"max_retries"`
	RetryBaseDelayMilliseconds int        `json:"retry_base_delay_milliseconds"`
	Status                     string     `json:"status"`
	NextRunAt                  time.Time  `json:"next_run_at"`
	LastScheduledAt            *time.Time `json:"last_scheduled_at"`
	CreatedBy                  uint64     `json:"created_by"`
	UpdatedBy                  uint64     `json:"updated_by"`
	CreatedAt                  time.Time  `json:"created_at"`
	UpdatedAt                  time.Time  `json:"updated_at"`
}

func (Task) TableName() string { return "probe_tasks" }

type Execution struct {
	ID                   uint64     `gorm:"primaryKey" json:"id"`
	ExecutionID          string     `gorm:"column:execution_id;size:32;uniqueIndex" json:"execution_id"`
	TaskID               uint64     `json:"task_id"`
	ServiceID            uint64     `json:"service_id"`
	ProbeType            string     `json:"probe_type"`
	TargetSnapshot       string     `json:"target_snapshot"`
	ScheduledAt          time.Time  `json:"scheduled_at"`
	StartedAt            *time.Time `json:"started_at"`
	FinishedAt           *time.Time `json:"finished_at"`
	DurationMilliseconds *int64     `json:"duration_milliseconds"`
	Status               string     `json:"status"`
	Success              *bool      `json:"success"`
	AttemptCount         int        `json:"attempt_count"`
	HTTPStatusCode       *int       `json:"http_status_code"`
	ErrorCode            *string    `json:"error_code"`
	ErrorMessage         *string    `json:"error_message"`
	WorkerConsumer       *string    `json:"worker_consumer"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

func (Execution) TableName() string { return "probe_results" }

type TaskInput struct {
	ServiceID                  uint64 `json:"service_id"`
	Name                       string `json:"name"`
	IntervalSeconds            int    `json:"interval_seconds"`
	TimeoutMilliseconds        int    `json:"timeout_milliseconds"`
	MaxRetries                 int    `json:"max_retries"`
	RetryBaseDelayMilliseconds int    `json:"retry_base_delay_milliseconds"`
}

type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}
type Page[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

type ExecutionMessage struct {
	ExecutionID string
	TaskID      uint64
	ScheduledAt time.Time
}

type ExecutionWork struct {
	Execution Execution
	Task      Task
}
