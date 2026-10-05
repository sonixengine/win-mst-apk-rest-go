package models

import (
	"time"
)

// LocalRole represents customizable user roles stored in SQLite
type LocalRole struct {
	ID               uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code             string    `gorm:"uniqueIndex;not null" json:"code"` // 'superadmin', 'admin', 'operator', etc.
	Name             string    `gorm:"not null" json:"name"`
	Description      string    `json:"description"`
	CanManageUsers   bool      `gorm:"default:false" json:"can_manage_users"`
	CanManageRoles   bool      `gorm:"default:false" json:"can_manage_roles"`
	CanViewWinlose   bool      `gorm:"default:false" json:"can_view_winlose"`
	CanManageDomains bool      `gorm:"default:true" json:"can_manage_domains"`
	CanUseSimulator  bool      `gorm:"default:true" json:"can_use_simulator"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	Users []LocalUser `gorm:"foreignKey:RoleID" json:"users,omitempty"`
}

func (LocalRole) TableName() string {
	return "local_roles"
}

// LocalUser represents a user account stored in the local SQLite database
type LocalUser struct {
	ID          uint       `gorm:"primaryKey;autoIncrement" json:"id"`
	UUID        string     `gorm:"uniqueIndex;not null" json:"uuid"`
	Name        string     `gorm:"not null" json:"name"`
	Email       string     `gorm:"uniqueIndex;not null" json:"email"`
	Password    string     `gorm:"not null" json:"-"` // bcrypt hash, hidden from JSON
	RoleID      uint       `gorm:"not null;index" json:"role_id"`
	IsActive    bool       `gorm:"default:true" json:"is_active"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	Role *LocalRole `gorm:"foreignKey:RoleID" json:"role,omitempty"`
}

func (LocalUser) TableName() string {
	return "local_users"
}
