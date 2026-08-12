package auth

import "time"

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

type User struct {
	ID           uint64     `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"size:100;not null;uniqueIndex" json:"username"`
	PasswordHash string     `gorm:"column:password_hash;size:255;not null" json:"-"`
	Status       string     `gorm:"size:20;not null" json:"status"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (User) TableName() string { return "users" }

type PublicUser struct {
	ID          uint64     `json:"id"`
	Username    string     `json:"username"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func publicUser(user User) PublicUser {
	return PublicUser{ID: user.ID, Username: user.Username, Status: user.Status, LastLoginAt: user.LastLoginAt, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
}
