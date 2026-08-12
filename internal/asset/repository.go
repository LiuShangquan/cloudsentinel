package asset

import (
	"context"
	"errors"
	"strconv"

	"cloudsentinel/internal/audit"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var (
	ErrNotFound = errors.New("asset not found")
	ErrConflict = errors.New("asset conflict")
)

type Store interface {
	CreateHost(context.Context, *Host, audit.Log) error
	GetHost(context.Context, uint64) (Host, error)
	ListHosts(context.Context, int, int) ([]Host, int64, error)
	UpdateHost(context.Context, *Host, audit.Log) error
	DisableHost(context.Context, uint64, uint64, audit.Log) error
	CountActiveServices(context.Context, uint64) (int64, error)
	CreateService(context.Context, *MonitoredService, audit.Log) error
	GetService(context.Context, uint64) (MonitoredService, error)
	ListServices(context.Context, int, int) ([]MonitoredService, int64, error)
	UpdateService(context.Context, *MonitoredService, audit.Log) error
	DisableService(context.Context, uint64, uint64, audit.Log) error
	CountActiveProbeTasks(context.Context, uint64) (int64, error)
}

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) CreateHost(ctx context.Context, host *Host, entry audit.Log) error {
	return translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(host).Error; err != nil {
			return err
		}
		entry.ResourceID = stringPointer(strconv.FormatUint(host.ID, 10))
		return tx.Create(&entry).Error
	}))
}

func (r *Repository) GetHost(ctx context.Context, id uint64) (Host, error) {
	var host Host
	if err := r.db.WithContext(ctx).First(&host, id).Error; err != nil {
		return Host{}, translate(err)
	}
	return host, nil
}

func (r *Repository) ListHosts(ctx context.Context, page, size int) ([]Host, int64, error) {
	var items []Host
	var total int64
	db := r.db.WithContext(ctx).Model(&Host{})
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) UpdateHost(ctx context.Context, host *Host, entry audit.Log) error {
	return translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Host{}).Where("id = ?", host.ID).Updates(map[string]any{"name": host.Name, "address": host.Address, "description": host.Description, "updated_by": host.UpdatedBy})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return tx.Create(&entry).Error
	}))
}

func (r *Repository) DisableHost(ctx context.Context, id, actor uint64, entry audit.Log) error {
	return r.transition(ctx, &Host{}, id, actor, entry)
}

func (r *Repository) CountActiveServices(ctx context.Context, hostID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&MonitoredService{}).Where("host_id = ? AND status = ?", hostID, StatusActive).Count(&count).Error
	return count, err
}

func (r *Repository) CreateService(ctx context.Context, service *MonitoredService, entry audit.Log) error {
	return translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(service).Error; err != nil {
			return err
		}
		entry.ResourceID = stringPointer(strconv.FormatUint(service.ID, 10))
		return tx.Create(&entry).Error
	}))
}

func (r *Repository) GetService(ctx context.Context, id uint64) (MonitoredService, error) {
	var service MonitoredService
	if err := r.db.WithContext(ctx).First(&service, id).Error; err != nil {
		return MonitoredService{}, translate(err)
	}
	return service, nil
}

func (r *Repository) ListServices(ctx context.Context, page, size int) ([]MonitoredService, int64, error) {
	var items []MonitoredService
	var total int64
	db := r.db.WithContext(ctx).Model(&MonitoredService{})
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) UpdateService(ctx context.Context, service *MonitoredService, entry audit.Log) error {
	return translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&MonitoredService{}).Where("id = ?", service.ID).Updates(map[string]any{"host_id": service.HostID, "name": service.Name, "type": service.Type, "target": service.Target, "description": service.Description, "updated_by": service.UpdatedBy})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return tx.Create(&entry).Error
	}))
}

func (r *Repository) DisableService(ctx context.Context, id, actor uint64, entry audit.Log) error {
	return r.transition(ctx, &MonitoredService{}, id, actor, entry)
}

func (r *Repository) CountActiveProbeTasks(ctx context.Context, serviceID uint64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("probe_tasks").Where("service_id = ? AND status = ?", serviceID, StatusActive).Count(&count).Error
	return count, err
}

func (r *Repository) transition(ctx context.Context, model any, id, actor uint64, entry audit.Log) error {
	return translate(r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(model).Where("id = ?", id).Updates(map[string]any{"status": StatusDisabled, "updated_by": actor})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return tx.Create(&entry).Error
	}))
}

func translate(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	var mysqlError *mysql.MySQLError
	if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
		return ErrConflict
	}
	return err
}

func stringPointer(value string) *string { return &value }
