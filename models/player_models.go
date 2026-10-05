package models

import "time"

// PlayerUser represents a player record in the agent's MySQL database
type PlayerUser struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `json:"username"`
	ExtPlayer string    `gorm:"column:extplayer" json:"extplayer"`
	AccName   string    `gorm:"column:accName" json:"acc_name"`
	AccNumber string    `gorm:"column:accNumber" json:"acc_number"`
	Bank      string    `gorm:"column:bank" json:"bank"`
	Saldo     float64   `gorm:"column:saldo" json:"saldo"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (PlayerUser) TableName() string {
	return "users"
}

// PlayerTransaction represents transaction history in the agent's MySQL database
type PlayerTransaction struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"column:user_id" json:"user_id"`
	Type      string    `gorm:"column:type" json:"type"` // deposit, withdraw, bet, win
	Amount    float64   `gorm:"column:amount" json:"amount"`
	Status    string    `gorm:"column:status" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (PlayerTransaction) TableName() string {
	return "transactions"
}

// TenantPlayerSummary provides high-level player metrics for an agent
type TenantPlayerSummary struct {
	TotalPlayers  int64   `json:"total_players"`
	TotalSaldo    float64 `json:"total_saldo"`
	ActiveToday   int64   `json:"active_today"`
	DepositToday  float64 `json:"deposit_today"`
	WithdrawToday float64 `json:"withdraw_today"`
}
