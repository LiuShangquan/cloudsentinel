package probe

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"cloudsentinel/internal/asset"
	"cloudsentinel/internal/audit"
)

var ErrInvalidInput = errors.New("invalid probe input")

type Metadata struct{ RequestID, ClientIP, UserAgent string }
type TaskService struct {
	repo *Repository
	now  func() time.Time
}

func NewTaskService(repo *Repository) *TaskService { return &TaskService{repo: repo, now: time.Now} }
func (s *TaskService) Create(ctx context.Context, input TaskInput, actor uint64, metadata Metadata) (Task, error) {
	service, err := s.repo.GetService(ctx, input.ServiceID)
	if err != nil {
		return Task{}, err
	}
	if service.Status != asset.StatusActive {
		return Task{}, ErrConflict
	}
	if err := validateTask(input); err != nil {
		return Task{}, err
	}
	now := s.now().UTC()
	task := Task{ServiceID: input.ServiceID, Name: strings.TrimSpace(input.Name), IntervalSeconds: input.IntervalSeconds, TimeoutMilliseconds: input.TimeoutMilliseconds, MaxRetries: input.MaxRetries, RetryBaseDelayMilliseconds: input.RetryBaseDelayMilliseconds, Status: TaskActive, NextRunAt: now, CreatedBy: actor, UpdatedBy: actor}
	if err := s.repo.CreateTask(ctx, &task, taskAudit(actor, "probe_task.create", 0, metadata)); err != nil {
		return Task{}, err
	}
	return task, nil
}
func (s *TaskService) Get(ctx context.Context, id uint64) (Task, error) {
	return s.repo.GetTask(ctx, id)
}
func (s *TaskService) List(ctx context.Context, page, size int) (Page[Task], error) {
	items, total, err := s.repo.ListTasks(ctx, page, size)
	if err != nil {
		return Page[Task]{}, err
	}
	return Page[Task]{Items: items, Pagination: pageInfo(page, size, total)}, nil
}
func (s *TaskService) Update(ctx context.Context, id uint64, input TaskInput, actor uint64, metadata Metadata) (Task, error) {
	current, err := s.repo.GetTask(ctx, id)
	if err != nil {
		return Task{}, err
	}
	service, err := s.repo.GetService(ctx, input.ServiceID)
	if err != nil {
		return Task{}, err
	}
	if service.Status != asset.StatusActive || current.Status != TaskActive {
		return Task{}, ErrConflict
	}
	if err := validateTask(input); err != nil {
		return Task{}, err
	}
	task := Task{ID: id, ServiceID: input.ServiceID, Name: strings.TrimSpace(input.Name), IntervalSeconds: input.IntervalSeconds, TimeoutMilliseconds: input.TimeoutMilliseconds, MaxRetries: input.MaxRetries, RetryBaseDelayMilliseconds: input.RetryBaseDelayMilliseconds, UpdatedBy: actor}
	if err := s.repo.UpdateTask(ctx, &task, taskAudit(actor, "probe_task.update", id, metadata)); err != nil {
		return Task{}, err
	}
	return s.repo.GetTask(ctx, id)
}
func (s *TaskService) Disable(ctx context.Context, id, actor uint64, metadata Metadata) error {
	if _, err := s.repo.GetTask(ctx, id); err != nil {
		return err
	}
	return s.repo.DisableTask(ctx, id, actor, taskAudit(actor, "probe_task.disable", id, metadata))
}
func (s *TaskService) ListResults(ctx context.Context, page, size int) (Page[Execution], error) {
	items, total, err := s.repo.ListExecutions(ctx, page, size)
	if err != nil {
		return Page[Execution]{}, err
	}
	return Page[Execution]{Items: items, Pagination: pageInfo(page, size, total)}, nil
}
func (s *TaskService) GetResult(ctx context.Context, id uint64) (Execution, error) {
	return s.repo.GetExecutionByID(ctx, id)
}
func validateTask(input TaskInput) error {
	if strings.TrimSpace(input.Name) == "" || len(strings.TrimSpace(input.Name)) > 100 || input.ServiceID == 0 || input.IntervalSeconds < 10 || input.IntervalSeconds > 86400 || input.TimeoutMilliseconds < 100 || input.TimeoutMilliseconds > 60000 || input.MaxRetries < 0 || input.MaxRetries > 5 || input.RetryBaseDelayMilliseconds < 100 || input.RetryBaseDelayMilliseconds > 30000 {
		return ErrInvalidInput
	}
	return nil
}
func pageInfo(page, size int, total int64) Pagination {
	pages := int64(0)
	if total > 0 {
		pages = (total + int64(size) - 1) / int64(size)
	}
	return Pagination{Page: page, PageSize: size, Total: total, TotalPages: pages}
}
func taskAudit(actor uint64, action string, id uint64, metadata Metadata) audit.Log {
	resource := "probe_task"
	outcome := "success"
	requestID := metadata.RequestID
	clientIP := metadata.ClientIP
	userAgent := metadata.UserAgent
	entry := audit.Log{ActorUserID: &actor, Action: action, Outcome: outcome, ResourceType: &resource, RequestID: &requestID, ClientIP: &clientIP, UserAgent: &userAgent}
	if id > 0 {
		value := strconv.FormatUint(id, 10)
		entry.ResourceID = &value
	}
	return entry
}
