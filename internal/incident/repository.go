package incident

import (
	"cloudsentinel/internal/audit"
	"context"
	"errors"
	"gorm.io/gorm"
	"time"
)

var (
	ErrNotFound = errors.New("incident not found")
	ErrConflict = errors.New("incident transition conflict")
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) UpsertFiring(ctx context.Context, value Incident) error {
	sql := `INSERT INTO incidents (event_key,fingerprint,external_fingerprint,alert_name,service_id,task_id,probe_type,severity,status,summary,description,fired_at,last_seen_at,occurrence_count,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,1,UTC_TIMESTAMP(3),UTC_TIMESTAMP(3)) ON DUPLICATE KEY UPDATE occurrence_count=occurrence_count+1,last_seen_at=VALUES(last_seen_at),updated_at=UTC_TIMESTAMP(3)`
	return r.db.WithContext(ctx).Exec(sql, value.EventKey, value.Fingerprint, value.ExternalFingerprint, value.AlertName, value.ServiceID, value.TaskID, value.ProbeType, value.Severity, StatusFiring, value.Summary, value.Description, value.FiredAt, value.LastSeenAt).Error
}
func (r *Repository) Resolve(ctx context.Context, value Incident) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Incident{}).Where("event_key = ?", value.EventKey).Updates(map[string]any{"status": gorm.Expr("CASE WHEN status = 'closed' THEN status ELSE 'resolved' END"), "resolved_at": gorm.Expr("COALESCE(resolved_at, ?)", value.ResolvedAt), "last_seen_at": value.LastSeenAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			return nil
		}
		return tx.Create(&value).Error
	})
}
func (r *Repository) Get(ctx context.Context, id uint64) (Incident, error) {
	var value Incident
	if err := r.db.WithContext(ctx).First(&value, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return value, ErrNotFound
		}
		return value, err
	}
	return value, nil
}
func (r *Repository) List(ctx context.Context, page, size int) ([]Incident, int64, error) {
	var items []Incident
	var total int64
	db := r.db.WithContext(ctx).Model(&Incident{})
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
func (r *Repository) Transition(ctx context.Context, id uint64, from []string, to string, actor uint64, entry audit.Log) (Incident, error) {
	var value Incident
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"status": to}
		now := time.Now().UTC()
		switch to {
		case StatusAcknowledged:
			updates["acknowledged_at"] = now
			updates["acknowledged_by"] = actor
		case StatusProcessing:
			updates["processing_at"] = now
			updates["processing_by"] = actor
		case StatusClosed:
			updates["closed_at"] = now
			updates["closed_by"] = actor
		}
		result := tx.Model(&Incident{}).Where("id = ? AND status IN ?", id, from).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrConflict
		}
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		return tx.First(&value, id).Error
	})
	return value, err
}
