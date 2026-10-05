package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"master-panel-api/database"
	"master-panel-api/middleware"
	"master-panel-api/models"

	"github.com/gofiber/fiber/v2"
)

// WinloseCategoryBreakdown represents turnover and win/loss per game category
type WinloseCategoryBreakdown struct {
	GameCategory  string  `json:"game_category"`
	TotalBetround int64   `json:"total_betround"`
	Turnover      float64 `json:"turnover"`
	Win           float64 `json:"win"`
	Profit        float64 `json:"profit"` // Turnover - Win (Company profit/winlose)
}

// AgentWinloseReport represents the comprehensive financial and game winlose report for an agent
type AgentWinloseReport struct {
	AgentUUID          string                     `json:"agent_uuid"`
	AgentCode          string                     `json:"agent_code"`
	AgentName          string                     `json:"agent_name"`
	StartDate          string                     `json:"start_date"`
	EndDate            string                     `json:"end_date"`
	TotalDeposit       float64                    `json:"total_deposit"`
	DepositCount       int64                      `json:"deposit_count"`
	TotalWithdraw      float64                    `json:"total_withdraw"`
	WithdrawCount      int64                      `json:"withdraw_count"`
	FinancialWinlose   float64                    `json:"financial_winlose"` // TotalDeposit - TotalWithdraw
	TotalTurnover      float64                    `json:"total_turnover"`
	TotalWin           float64                    `json:"total_win"`
	GameWinlose        float64                    `json:"game_winlose"` // TotalTurnover - TotalWin
	TotalBetround      int64                      `json:"total_betround"`
	ActivePlayersCount int64                      `json:"active_players_count"`
	CategoryBreakdown  []WinloseCategoryBreakdown `json:"category_breakdown"`
	Status             string                     `json:"status"` // "ok", "db_error", "no_data"
	Cached             bool                       `json:"cached"`
	ErrorMessage       string                     `json:"error_message,omitempty"`
}

// ReportAgentItem represents basic agent metadata for report listing
type ReportAgentItem struct {
	UUID          string `json:"uuid"`
	AgentCode     string `json:"agent_code"`
	Name          string `json:"name"`
	MasterAgentID uint   `json:"master_agent_id"`
	IsActive      bool   `json:"is_active"`
}

// ListReportAgents returns accessible ACTIVE agents with pagination for the winlose report dashboard
func ListReportAgents(c *fiber.Ctx) error {
	claims, _ := c.Locals("user").(*middleware.UserClaims)

	if database.AdminDB == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"message": "Database not initialized",
		})
	}

	query := database.AdminDB.Model(&models.Agent{}).Where("is_active = ?", true)

	if !claims.IsSuperMaster() {
		if claims != nil && claims.MasterAgentID != nil {
			query = query.Where("master_agent_id = ?", *claims.MasterAgentID)
		} else {
			return c.JSON(fiber.Map{
				"success": true,
				"data":    []ReportAgentItem{},
				"pagination": fiber.Map{
					"page":        1,
					"limit":       10,
					"total":       0,
					"total_pages": 0,
				},
			})
		}
	}

	// Search filter
	search := strings.TrimSpace(c.Query("search"))
	if search != "" {
		lowerSearch := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(agent_code) LIKE ? OR LOWER(name) LIKE ?", lowerSearch, lowerSearch)
	}

	// Pagination params
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	limit := c.QueryInt("limit", 10)
	if limit < 1 {
		limit = 10
	} else if limit > 100 {
		limit = 100
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to count active agents",
			"error":   err.Error(),
		})
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	if totalPages < 0 {
		totalPages = 0
	}

	offset := (page - 1) * limit
	var agents []models.Agent
	if err := query.Order("agent_code ASC").Offset(offset).Limit(limit).Find(&agents).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch agents for report",
			"error":   err.Error(),
		})
	}

	result := make([]ReportAgentItem, len(agents))
	for i, ag := range agents {
		result[i] = ReportAgentItem{
			UUID:          ag.UUID,
			AgentCode:     ag.AgentCode,
			Name:          ag.Name,
			MasterAgentID: ag.MasterAgentID,
			IsActive:      ag.IsActive,
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
		"pagination": fiber.Map{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// GetAgentWinloseReport fetches financial and game winlose report for a single agent from its MySQL tenant DB
// Designed to be called asynchronously via lazy loading so client databases are not overwhelmed.
func GetAgentWinloseReport(c *fiber.Ctx) error {
	claims, _ := c.Locals("user").(*middleware.UserClaims)
	agentUUID := strings.TrimSpace(c.Params("uuid"))

	if err := checkPlayerTenantAccess(claims, agentUUID); err != nil {
		if fErr, ok := err.(*fiber.Error); ok {
			return c.Status(fErr.Code).JSON(fiber.Map{"success": false, "message": fErr.Message})
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	// 1. Get agent metadata
	if database.AdminDB == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"message": "Database not initialized",
		})
	}
	var agent models.Agent
	if err := database.AdminDB.Where("uuid = ?", agentUUID).First(&agent).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Agent not found",
		})
	}

	// Parse date range (default to today)
	todayStr := time.Now().Format("2006-01-02")
	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))

	if startDate == "" {
		startDate = todayStr
	}
	if endDate == "" {
		endDate = todayStr
	}

	// Cache key & force refresh flag
	cacheKey := fmt.Sprintf("winlose:%s:%s:%s", agentUUID, startDate, endDate)
	forceRefresh := c.Query("refresh") == "true"

	if !forceRefresh {
		if cachedJSON, found := database.GetReportCache(cacheKey); found {
			var cachedReport AgentWinloseReport
			if err := json.Unmarshal([]byte(cachedJSON), &cachedReport); err == nil {
				cachedReport.Cached = true
				return c.JSON(fiber.Map{
					"success": true,
					"data":    cachedReport,
					"cached":  true,
				})
			}
		}
	}

	report := AgentWinloseReport{
		AgentUUID:         agent.UUID,
		AgentCode:         agent.AgentCode,
		AgentName:         agent.Name,
		StartDate:         startDate,
		EndDate:           endDate,
		CategoryBreakdown: []WinloseCategoryBreakdown{},
		Status:            "ok",
		Cached:            false,
	}

	// 2. Connect to agent read-only MySQL database
	tenantDB, err := database.GetTenantDB(agentUUID, true)
	if err != nil {
		report.Status = "db_error"
		report.ErrorMessage = "Database agen tidak dapat dijangkau: " + err.Error()
		return c.JSON(fiber.Map{
			"success": true,
			"data":    report,
		})
	}

	// 3. Query Financial Data from transaksis table
	// type = 1 (deposit), type = 2 (withdraw), status_id = 2 (approved)
	startDateTime := startDate + " 00:00:00"
	endDateTime := endDate + " 23:59:59"

	type FinancialRow struct {
		TotalDeposit  float64 `gorm:"column:total_deposit"`
		DepositCount  int64   `gorm:"column:deposit_count"`
		TotalWithdraw float64 `gorm:"column:total_withdraw"`
		WithdrawCount int64   `gorm:"column:withdraw_count"`
	}

	var finRow FinancialRow
	financialSQL := `
		SELECT 
			COALESCE(SUM(CASE WHEN type = 1 AND status_id = 2 AND payment_category_id IN (1, 2, 3, 4, 5) THEN amount ELSE 0 END), 0) AS total_deposit,
			COUNT(CASE WHEN type = 1 AND status_id = 2 AND payment_category_id IN (1, 2, 3, 4, 5) THEN 1 END) AS deposit_count,
			COALESCE(SUM(CASE WHEN type = 2 AND status_id = 2 AND (payment_category_id IS NULL OR payment_category_id IN (5)) THEN amount ELSE 0 END), 0) AS total_withdraw,
			COUNT(CASE WHEN type = 2 AND status_id = 2 AND (payment_category_id IS NULL OR payment_category_id IN (5)) THEN 1 END) AS withdraw_count
		FROM transaksis
		WHERE created_at BETWEEN ? AND ?
	`
	if err := tenantDB.Raw(financialSQL, startDateTime, endDateTime).Scan(&finRow).Error; err != nil {
		// If transaksis table has issues, log but continue gracefully
		report.ErrorMessage = "Peringatan transaksi: " + err.Error()
	} else {
		report.TotalDeposit = finRow.TotalDeposit
		report.DepositCount = finRow.DepositCount
		report.TotalWithdraw = finRow.TotalWithdraw
		report.WithdrawCount = finRow.WithdrawCount
		report.FinancialWinlose = finRow.TotalDeposit - finRow.TotalWithdraw
	}

	// 4. Query Game Winlose from user_rebate_transaction table (as in fetchReportsLogic)
	type CategoryRow struct {
		GameCategory  string  `gorm:"column:game_category"`
		TotalBetround int64   `gorm:"column:total_betround"`
		Turnover      float64 `gorm:"column:turnover"`
		Win           float64 `gorm:"column:win"`
		Profit        float64 `gorm:"column:profit"`
	}

	var catRows []CategoryRow
	gameCategorySQL := `
		SELECT 
			COALESCE(NULLIF(game_category, ''), 'other') AS game_category,
			COALESCE(SUM(total_betround), 0) AS total_betround,
			COALESCE(SUM(total_turnover), 0) AS turnover,
			COALESCE(SUM(total_win), 0) AS win,
			COALESCE(SUM(total_turnover - total_win), 0) AS profit
		FROM user_rebate_transaction
		WHERE transaction_date >= ? AND transaction_date <= ?
		GROUP BY game_category
		ORDER BY turnover DESC
	`
	if err := tenantDB.Raw(gameCategorySQL, startDate, endDate).Scan(&catRows).Error; err != nil {
		// Table user_rebate_transaction may not be present in newer or unmigrated tenants; graceful fallback
		if report.ErrorMessage != "" {
			report.ErrorMessage += " | Peringatan game rebate: " + err.Error()
		} else {
			report.ErrorMessage = "Data game rebate tidak ditemukan: " + err.Error()
		}
	} else {
		var sumTurnover, sumWin, sumProfit float64
		var sumBetround int64
		breakdowns := make([]WinloseCategoryBreakdown, len(catRows))

		for i, cr := range catRows {
			breakdowns[i] = WinloseCategoryBreakdown{
				GameCategory:  cr.GameCategory,
				TotalBetround: cr.TotalBetround,
				Turnover:      cr.Turnover,
				Win:           cr.Win,
				Profit:        cr.Profit,
			}
			sumTurnover += cr.Turnover
			sumWin += cr.Win
			sumProfit += cr.Profit
			sumBetround += cr.TotalBetround
		}

		report.TotalTurnover = sumTurnover
		report.TotalWin = sumWin
		report.GameWinlose = sumProfit
		report.TotalBetround = sumBetround
		report.CategoryBreakdown = breakdowns
	}

	// 5. Count active players (distinct user_id in user_rebate_transaction)
	var activePlayers int64
	activePlayersSQL := `
		SELECT COUNT(DISTINCT user_id)
		FROM user_rebate_transaction
		WHERE transaction_date >= ? AND transaction_date <= ?
	`
	_ = tenantDB.Raw(activePlayersSQL, startDate, endDate).Scan(&activePlayers).Error
	report.ActivePlayersCount = activePlayers

	// Save to cache if status is ok
	if report.Status == "ok" {
		ttl := 5 * time.Minute
		if endDate < todayStr {
			ttl = 2 * time.Hour
		}
		if reportBytes, err := json.Marshal(report); err == nil {
			database.SetReportCache(cacheKey, string(reportBytes), ttl)
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    report,
		"cached":  false,
	})
}

// ActivePlayerItem represents a player's betting summary
type ActivePlayerItem struct {
	UserID        int64   `json:"user_id" gorm:"column:user_id"`
	Username      string  `json:"username" gorm:"column:username"`
	ExtPlayer     string  `json:"extplayer" gorm:"column:extplayer"`
	Saldo         float64 `json:"saldo" gorm:"column:saldo"`
	TotalBetround int64   `json:"total_betround" gorm:"column:total_betround"`
	Turnover      float64 `json:"turnover" gorm:"column:turnover"`
	Win           float64 `json:"win" gorm:"column:win"`
	Profit        float64 `json:"profit" gorm:"column:profit"` // Turnover - Win (Company profit/winlose)
}

// ActivePlayersResponse represents the paginated response for active players
type ActivePlayersResponse struct {
	AgentUUID  string             `json:"agent_uuid"`
	AgentCode  string             `json:"agent_code"`
	AgentName  string             `json:"agent_name"`
	StartDate  string             `json:"start_date"`
	EndDate    string             `json:"end_date"`
	Page       int                `json:"page"`
	Limit      int                `json:"limit"`
	TotalItems int64              `json:"total_items"`
	TotalPages int                `json:"total_pages"`
	Players    []ActivePlayerItem `json:"players"`
	Cached     bool               `json:"cached"`
}

// GetAgentActivePlayersReport retrieves the list of players who placed bets/played during the period for a specific agent
func GetAgentActivePlayersReport(c *fiber.Ctx) error {
	agentUUID := strings.TrimSpace(c.Params("uuid"))
	if agentUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Agent UUID wajib diisi",
		})
	}

	claims, _ := c.Locals("user").(*middleware.UserClaims)
	if err := checkPlayerTenantAccess(claims, agentUUID); err != nil {
		if fErr, ok := err.(*fiber.Error); ok {
			return c.Status(fErr.Code).JSON(fiber.Map{"success": false, "message": fErr.Message})
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	if database.AdminDB == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"message": "Database not initialized",
		})
	}

	var agent models.Agent
	if err := database.AdminDB.Where("uuid = ?", agentUUID).First(&agent).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent tidak ditemukan",
		})
	}

	loc, _ := time.LoadLocation("Asia/Jakarta")
	todayStr := time.Now().In(loc).Format("2006-01-02")
	startDate := c.Query("start_date", todayStr)
	endDate := c.Query("end_date", todayStr)
	search := strings.TrimSpace(c.Query("search", ""))
	page := c.QueryInt("page", 1)
	if page < 1 {
		page = 1
	}
	limit := c.QueryInt("limit", 15)
	if limit < 1 || limit > 100 {
		limit = 15
	}
	offset := (page - 1) * limit
	forceRefresh := c.Query("refresh") == "true"

	cacheKey := fmt.Sprintf("winlose_players:%s:%s:%s:%d:%d:%s", agentUUID, startDate, endDate, page, limit, search)

	// Check cache if not forcing refresh
	if !forceRefresh {
		if cachedData, found := database.GetReportCache(cacheKey); found {
			var resp ActivePlayersResponse
			if err := json.Unmarshal([]byte(cachedData), &resp); err == nil {
				resp.Cached = true
				return c.JSON(fiber.Map{
					"success": true,
					"data":    resp,
					"cached":  true,
				})
			}
		}
	}

	// Connect to tenant DB (read-only pool)
	tenantDB, err := database.GetTenantDB(agentUUID, true)
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "Koneksi ke database agen gagal: " + err.Error(),
		})
	}

	// Build WHERE clause
	whereClause := "urt.transaction_date >= ? AND urt.transaction_date <= ?"
	params := []interface{}{startDate, endDate}

	if search != "" {
		whereClause += " AND (users.username LIKE ? OR users.extplayer LIKE ?)"
		searchParam := "%" + search + "%"
		params = append(params, searchParam, searchParam)
	}

	// Count total active players matching filters
	countSQL := fmt.Sprintf(`
		SELECT COUNT(DISTINCT urt.user_id)
		FROM user_rebate_transaction urt
		LEFT JOIN users ON urt.user_id = users.id
		WHERE %s
	`, whereClause)

	var totalItems int64
	if err := tenantDB.Raw(countSQL, params...).Scan(&totalItems).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal membaca data pemain: " + err.Error(),
		})
	}

	// Query paginated players
	dataSQL := fmt.Sprintf(`
		SELECT 
			urt.user_id,
			COALESCE(users.username, '') AS username,
			COALESCE(users.extplayer, '') AS extplayer,
			COALESCE(users.saldo, 0) AS saldo,
			COALESCE(SUM(urt.total_betround), 0) AS total_betround,
			COALESCE(SUM(urt.total_turnover), 0) AS turnover,
			COALESCE(SUM(urt.total_win), 0) AS win,
			COALESCE(SUM(urt.total_turnover - urt.total_win), 0) AS profit
		FROM user_rebate_transaction urt
		LEFT JOIN users ON urt.user_id = users.id
		WHERE %s
		GROUP BY urt.user_id, users.username, users.extplayer, users.saldo
		ORDER BY turnover DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	queryParams := append(params, limit, offset)
	var players []ActivePlayerItem
	if err := tenantDB.Raw(dataSQL, queryParams...).Scan(&players).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Gagal membaca detail pemain: " + err.Error(),
		})
	}

	totalPages := 0
	if totalItems > 0 {
		totalPages = int((totalItems + int64(limit) - 1) / int64(limit))
	}

	resp := ActivePlayersResponse{
		AgentUUID:  agent.UUID,
		AgentCode:  agent.AgentCode,
		AgentName:  agent.Name,
		StartDate:  startDate,
		EndDate:    endDate,
		Page:       page,
		Limit:      limit,
		TotalItems: totalItems,
		TotalPages: totalPages,
		Players:    players,
		Cached:     false,
	}

	// Save to cache
	ttl := 5 * time.Minute
	if endDate < todayStr {
		ttl = 2 * time.Hour
	}
	if respBytes, err := json.Marshal(resp); err == nil {
		database.SetReportCache(cacheKey, string(respBytes), ttl)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    resp,
		"cached":  false,
	})
}
