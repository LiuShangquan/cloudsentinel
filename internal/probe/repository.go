package probe

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"cloudsentinel/internal/asset"
	"cloudsentinel/internal/audit"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNotFound = errors.New("probe resource not found")
	ErrConflict = errors.New("probe resource conflict")
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) GetService(ctx context.Context, id uint64) (asset.MonitoredService, error) {
	var value asset.MonitoredService
	if err := r.db.WithContext(ctx).First(&value, id).Error; err != nil {
		return value, translate(err)
	}
	return value, nil
}
func (r *Repository) CreateTask(ctx context.Context, task *Task, entry audit.Log) error {
	return translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(task).Error; err != nil {
			return err
		}
		id := strconv.FormatUint(task.ID, 10)
		entry.ResourceID = &id
		return tx.Create(&entry).Error
	}))
}
func (r *Repository) GetTask(ctx context.Context, id uint64) (Task, error) {
	var value Task
	if err := r.db.WithContext(ctx).First(&value, id).Error; err != nil {
		return value, translate(err)
	}
	return value, nil
}
func (r *Repository) ListTasks(ctx context.Context, page, size int) ([]Task, int64, error) {
	var items []Task
	var total int64
	db := r.db.WithContext(ctx).Model(&Task{})
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
func (r *Repository) UpdateTask(ctx context.Context, task *Task, entry audit.Log) error {
	return translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Task{}).Where("id = ?", task.ID).Updates(map[string]any{"service_id": task.ServiceID, "name": task.Name, "interval_seconds": task.IntervalSeconds, "timeout_milliseconds": task.TimeoutMilliseconds, "max_retries": task.MaxRetries, "retry_base_delay_milliseconds": task.RetryBaseDelayMilliseconds, "updated_by": task.UpdatedBy})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return tx.Create(&entry).Error
	}))
}
func (r *Repository) DisableTask(ctx context.Context, id, actor uint64, entry audit.Log) error {
	return translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Task{}).Where("id = ?", id).Updates(map[string]any{"status": TaskDisabled, "updated_by": actor})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return tx.Create(&entry).Error
	}))
}
func (r *Repository) ListExecutions(ctx context.Context, page, size int) ([]Execution, int64, error) {
	var items []Execution
	var total int64
	db := r.db.WithContext(ctx).Model(&Execution{})
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
func (r *Repository) GetExecutionByID(ctx context.Context, id uint64) (Execution, error) {
	var value Execution
	if err := r.db.WithContext(ctx).First(&value, id).Error; err != nil {
		return value, translate(err)
	}
	return value, nil
}
func (r *Repository) GetWork(ctx context.Context, executionID string) (ExecutionWork, error) {
	var execution Execution
	if err := r.db.WithContext(ctx).Where("execution_id = ?", executionID).First(&execution).Error; err != nil {
		return ExecutionWork{}, translate(err)
	}
	var task Task
	if err := r.db.WithContext(ctx).First(&task, execution.TaskID).Error; err != nil {
		return ExecutionWork{}, translate(err)
	}
	return ExecutionWork{Execution: execution, Task: task}, nil
}

type dueRow struct {
	Task
	ServiceType string `gorm:"column:service_type"`
	Target      string `gorm:"column:target"`
}

func (r *Repository) ScheduleDue(ctx context.Context, batch int, now time.Time) ([]ExecutionMessage, error) {
	var messages []ExecutionMessage
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []dueRow
		query := tx.Table("probe_tasks AS pt").Select("pt.*, s.type AS service_type, s.target AS target").Joins("JOIN services s ON s.id = pt.service_id").Where("pt.status = ? AND s.status = ? AND pt.next_run_at <= ?", TaskActive, asset.StatusActive, now).Order("pt.next_run_at, pt.id").Limit(batch).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		if err := query.Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			id, err := executionID()
			if err != nil {
				return err
			}
			scheduled := row.NextRunAt
			execution := Execution{ExecutionID: id, TaskID: row.ID, ServiceID: row.ServiceID, ProbeType: row.ServiceType, TargetSnapshot: row.Target, ScheduledAt: scheduled, Status: ExecutionQueued}
			if err := tx.Create(&execution).Error; err != nil {
				return err
			}
			next := now.Add(time.Duration(row.IntervalSeconds) * time.Second)
			if err := tx.Model(&Task{}).Where("id = ?", row.ID).Updates(map[string]any{"last_scheduled_at": scheduled, "next_run_at": next}).Error; err != nil {
				return err
			}
			messages = append(messages, ExecutionMessage{ExecutionID: id, TaskID: row.ID, ScheduledAt: scheduled})
		}
		return nil
	})
	return messages, err
}
func (r *Repository) QueuedBefore(ctx context.Context, before time.Time, limit int) ([]ExecutionMessage, error) {
	var rows []Execution
	if err := r.db.WithContext(ctx).Where("status = ? AND scheduled_at < ?", ExecutionQueued, before).Order("scheduled_at").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	messages := make([]ExecutionMessage, 0, len(rows))
	for _, row := range rows {
		messages = append(messages, ExecutionMessage{ExecutionID: row.ExecutionID, TaskID: row.TaskID, ScheduledAt: row.ScheduledAt})
	}
	return messages, nil
}
func (r *Repository) Acquire(ctx context.Context, id, consumer string, started, staleBefore time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&Execution{}).Where("execution_id = ? AND (status = ? OR (status = ? AND updated_at < ?))", id, ExecutionQueued, ExecutionRunning, staleBefore).Updates(map[string]any{"status": ExecutionRunning, "started_at": started, "worker_consumer": consumer})
	return result.RowsAffected == 1, result.Error
}
func (r *Repository) MarkFinal(ctx context.Context, id, consumer string, finished time.Time, duration int64, result Result, attempts int) error {
	status := ExecutionFailed
	if result.Success {
		status = ExecutionSuccess
	}
	values := map[string]any{"status": status, "success": result.Success, "finished_at": finished, "duration_milliseconds": duration, "attempt_count": attempts, "http_status_code": nullableInt(result.HTTPStatusCode), "error_code": nullableString(result.ErrorCode), "error_message": nullableString(truncate(result.ErrorMessage, 1024))}
	update := r.db.WithContext(ctx).Model(&Execution{}).Where("execution_id = ? AND status = ? AND worker_consumer = ?", id, ExecutionRunning, consumer).Updates(values)
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return ErrConflict
	}
	return nil
}

func executionID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}
func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func nullableInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}
func truncate(value string, max int) string {
	if len(value) > max {
		return value[:max]
	}
	return value
}
func translate(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return ErrConflict
	}
	return err
}
