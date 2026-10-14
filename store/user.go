package store

import (
	"time"

	"github.com/vvb13a/godam/domain"
)

// User mirrors the "users" table. PasswordHash holds a bcrypt hash; it is kept
// out of the domain model so it is never serialized.
type User struct {
	ID           string    `gorm:"column:id;primaryKey;type:text"`
	Name         string    `gorm:"column:name;type:text;not null"`
	Email        string    `gorm:"column:email;type:text;not null;uniqueIndex"`
	PasswordHash string    `gorm:"column:password_hash;type:text;not null;default:''"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (User) TableName() string { return "users" }

func UserModel(u *domain.User) *User {
	return &User{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func (m *User) ToDomain() *domain.User {
	return &domain.User{
		ID:        m.ID,
		Name:      m.Name,
		Email:     m.Email,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
