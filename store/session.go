package store

import "time"

// Session mirrors the "sessions" table; the token is the opaque cookie value.
type Session struct {
	Token     string    `gorm:"column:token;primaryKey;type:text"`
	UserID    string    `gorm:"column:user_id;type:text;not null;index"`
	CreatedAt time.Time `gorm:"column:created_at"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
}

func (Session) TableName() string { return "sessions" }
