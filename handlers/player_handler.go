package handlers

import (
	"strconv"

	"master-panel-api/database"
	"master-panel-api/middleware"
	"master-panel-api/models"

	"github.com/gofiber/fiber/v2"
)

// checkPlayerTenantAccess ensures the user has permission to access the agent's player DB
func checkPlayerTenantAccess(claims *middleware.UserClaims, agentUUID string) error {
	if claims.IsSuperMaster() {
		return nil
	}
	if database.AdminDB == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "Database not initialized")
	}
	var agent models.Agent
	if err := database.AdminDB.Where("uuid = ?", agentUUID).First(&agent).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Agent not found")
	}
	if claims == nil || claims.MasterAgentID == nil || agent.MasterAgentID != *claims.MasterAgentID {
		return fiber.NewError(fiber.StatusForbidden, "Forbidden: You do not have access to this agent's database")
	}
	return nil
}

// GetAgentPlayerSummary queries the agent's MySQL player database to get aggregated metrics
func GetAgentPlayerSummary(c *fiber.Ctx) error {
	claims, _ := c.Locals("user").(*middleware.UserClaims)
	agentUUID := c.Params("uuid")

	if err := checkPlayerTenantAccess(claims, agentUUID); err != nil {
		if fErr, ok := err.(*fiber.Error); ok {
			return c.Status(fErr.Code).JSON(fiber.Map{"success": false, "message": fErr.Message})
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	// Connect to read-only MySQL tenant DB
	tenantDB, err := database.GetTenantDB(agentUUID, true)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Failed to connect to agent player database",
			"error":   err.Error(),
		})
	}

	var totalPlayers int64
	var totalSaldo float64

	tenantDB.Model(&models.PlayerUser{}).Count(&totalPlayers)
	tenantDB.Model(&models.PlayerUser{}).Select("COALESCE(SUM(saldo), 0)").Scan(&totalSaldo)

	summary := models.TenantPlayerSummary{
		TotalPlayers: totalPlayers,
		TotalSaldo:   totalSaldo,
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    summary,
	})
}

// ListAgentPlayers returns a paginated list of players from the agent's MySQL database
func ListAgentPlayers(c *fiber.Ctx) error {
	claims, _ := c.Locals("user").(*middleware.UserClaims)
	agentUUID := c.Params("uuid")

	if err := checkPlayerTenantAccess(claims, agentUUID); err != nil {
		if fErr, ok := err.(*fiber.Error); ok {
			return c.Status(fErr.Code).JSON(fiber.Map{"success": false, "message": fErr.Message})
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	search := c.Query("search", "")
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	tenantDB, err := database.GetTenantDB(agentUUID, true)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Failed to connect to agent player database",
			"error":   err.Error(),
		})
	}

	query := tenantDB.Model(&models.PlayerUser{})
	if search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("username LIKE ? OR extplayer LIKE ? OR accName LIKE ? OR accNumber LIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern)
	}

	var total int64
	query.Count(&total)

	var players []models.PlayerUser
	if err := query.Order("id DESC").Offset(offset).Limit(limit).Find(&players).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to query players",
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    players,
		"pagination": fiber.Map{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}
