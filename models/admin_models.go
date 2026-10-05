package models

import (
	"time"
)

// User represents an admin or backoffice user in PostgreSQL (engine-auth)
type User struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	UUID             string    `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name             string    `json:"name"`
	Email            string    `gorm:"uniqueIndex" json:"email"`
	Password         string    `json:"-"` // Hidden from JSON serialization
	TwoFAEnabled     bool      `json:"two_fa_enabled"`
	TwoFASecret      *string   `json:"-"`
	TwoFACompleted   bool      `json:"two_fa_completed"`
	IsMaster         bool      `json:"is_master"`
	UserGroupID      *uint     `json:"user_group_id"`
	MasterAgentID    *uint     `json:"master_agent_id"`
	IsActive         bool      `json:"is_active"`
	Internal         bool      `json:"internal"`
	TransactionLimit float64   `json:"transaction_limit"`
	JWTRefreshToken  *string   `json:"-"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	MasterAgent *AgentMaster `gorm:"foreignKey:MasterAgentID" json:"master_agent,omitempty"`
	UserGroup   *UserGroup   `gorm:"foreignKey:UserGroupID" json:"user_group,omitempty"`
}

func (User) TableName() string {
	return "users"
}

// AgentMaster represents a master agent group in PostgreSQL
type AgentMaster struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UUID       string    `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name       string    `json:"name"`
	MasterCode string    `json:"master_code"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
	Agents     []Agent   `gorm:"foreignKey:MasterAgentID" json:"agents,omitempty"`
}

func (AgentMaster) TableName() string {
	return "agent_master"
}

// Agent represents an individual agent/brand under a master agent in PostgreSQL
type Agent struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	UUID               string    `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	MasterAgentID      uint      `json:"master_agent_id"`
	Name               string    `json:"name"`
	AgentCode          string    `json:"agent_code"`
	AgentAPIToken      *string   `json:"agent_api_token,omitempty"`
	AgentSecret        *string   `json:"agent_secret,omitempty"`
	AgentAPIURL        *string   `json:"agent_api_url,omitempty"`
	AgentTelegramToken *string   `json:"agent_telegram_token,omitempty"`
	IsActive           bool      `json:"is_active"`
	CreatedAt          time.Time `json:"created_at,omitempty"`
	UpdatedAt          time.Time `json:"updated_at,omitempty"`

	Connection     *AgentConnection     `gorm:"foreignKey:AgentUUID;references:UUID" json:"connection,omitempty"`
	ConnectionRead *AgentConnectionRead `gorm:"foreignKey:AgentUUID;references:UUID" json:"connection_read,omitempty"`
}

func (Agent) TableName() string {
	return "agent"
}

// AgentConnection stores MySQL connection credentials for tenant player DB (Read/Write)
type AgentConnection struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	AgentUUID  string `gorm:"column:agent_uuid;index" json:"agent_uuid"`
	DBHost     string `gorm:"column:db_host" json:"db_host"`
	DBName     string `gorm:"column:db_name" json:"db_name"`
	DBUser     string `gorm:"column:db_user" json:"db_user"`
	DBPassword string `gorm:"column:db_password" json:"-"` // Hidden
	DBPort     int    `gorm:"column:db_port" json:"db_port"`
}

func (AgentConnection) TableName() string {
	return "agent_connection"
}

// AgentConnectionRead stores read-only MySQL credentials for tenant player DB
type AgentConnectionRead struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	AgentUUID  string `gorm:"column:agent_uuid;index" json:"agent_uuid"`
	DBHost     string `gorm:"column:db_host" json:"db_host"`
	DBName     string `gorm:"column:db_name" json:"db_name"`
	DBUser     string `gorm:"column:db_user" json:"db_user"`
	DBPassword string `gorm:"column:db_password" json:"-"` // Hidden
	DBPort     int    `gorm:"column:db_port" json:"db_port"`
}

func (AgentConnectionRead) TableName() string {
	return "agent_connection_read"
}

// UserGroup represents role groups in PostgreSQL
type UserGroup struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UUID         string    `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name         string    `json:"name"`
	IPWhitelist  *string   `json:"ip_whitelist"`
	EmailVisible bool      `json:"email_visible"`
	PhoneVisible bool      `json:"phone_visible"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

func (UserGroup) TableName() string {
	return "user_groups"
}

// MasterSiteConfig stores domain routing, maintenance, and branding for Cloudflare Worker & APK Gateway
type MasterSiteConfig struct {
	ID                 uint      `gorm:"primaryKey" json:"id"`
	AgentCode          string    `gorm:"uniqueIndex;not null" json:"agent_code"`
	AgentName          string    `json:"agent_name"`
	ActiveDomain       string    `json:"active_domain"`
	BackupDomains      string    `json:"backup_domains"` // Comma-separated or JSON list
	IsMaintenance      bool      `gorm:"default:false" json:"is_maintenance"`
	MaintenanceMessage string    `json:"maintenance_message"`
	AppName            string    `json:"app_name"`
	LogoURL            string    `json:"logo_url"`
	ApkDownloadURL     string    `json:"apk_download_url"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (MasterSiteConfig) TableName() string {
	return "master_site_configs"
}
