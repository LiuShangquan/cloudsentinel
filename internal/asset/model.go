package asset

import "time"

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
	TypeHTTP       = "http"
	TypeTCP        = "tcp"
)

type Host struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:100;not null;uniqueIndex" json:"name"`
	Address     string    `gorm:"size:255;not null" json:"address"`
	Description string    `gorm:"type:text;not null" json:"description"`
	Status      string    `gorm:"size:20;not null" json:"status"`
	CreatedBy   uint64    `json:"created_by"`
	UpdatedBy   uint64    `json:"updated_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Host) TableName() string { return "hosts" }

type MonitoredService struct {
	ID          uint64    `gorm:"primaryKey" json:"id"`
	HostID      uint64    `gorm:"column:host_id;not null;uniqueIndex:uk_services_host_name" json:"host_id"`
	Name        string    `gorm:"size:100;not null;uniqueIndex:uk_services_host_name" json:"name"`
	Type        string    `gorm:"size:20;not null" json:"type"`
	Target      string    `gorm:"size:2048;not null" json:"target"`
	Description string    `gorm:"type:text;not null" json:"description"`
	Status      string    `gorm:"size:20;not null" json:"status"`
	CreatedBy   uint64    `json:"created_by"`
	UpdatedBy   uint64    `json:"updated_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (MonitoredService) TableName() string { return "services" }

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
