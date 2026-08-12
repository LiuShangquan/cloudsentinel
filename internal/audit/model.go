package audit

import "time"

type Log struct {
	ID           uint64    `gorm:"primaryKey"`
	ActorUserID  *uint64   `gorm:"column:actor_user_id"`
	Username     *string   `gorm:"size:100"`
	Action       string    `gorm:"size:100;not null"`
	Outcome      string    `gorm:"size:32;not null"`
	FailureCode  *string   `gorm:"size:100"`
	ResourceType *string   `gorm:"size:100"`
	ResourceID   *string   `gorm:"size:100"`
	RequestID    *string   `gorm:"size:100"`
	ClientIP     *string   `gorm:"size:64"`
	UserAgent    *string   `gorm:"size:512"`
	Details      *string   `gorm:"type:json"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
}

func (Log) TableName() string { return "audit_logs" }
