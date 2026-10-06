package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"master-panel-api/config"
	"master-panel-api/database"
	"master-panel-api/middleware"
	"master-panel-api/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type AgentWithConfigResponse struct {
	models.Agent
	SiteConfig *models.MasterSiteConfig `json:"site_config"`
}

// checkAgentAccess verifies if the requesting user has permission for the agent
func checkAgentAccess(claims *middleware.UserClaims, agentCode string) error {
	if claims.IsSuperMaster() {
		return nil
	}
	var agent models.Agent
	if err := database.AdminDB.Where("LOWER(agent_code) = ?", strings.ToLower(agentCode)).First(&agent).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Agent not found")
	}
	if claims == nil || claims.MasterAgentID == nil || agent.MasterAgentID != *claims.MasterAgentID {
		return fiber.NewError(fiber.StatusForbidden, "Forbidden: You do not have access to this agent")
	}
	return nil
}

// ListAgents retrieves agents from PostgreSQL along with their master panel site configs.
// Supports lazy-loading filtering by status: ?status=active (is_active = true) or ?status=inactive (is_active = false)
// Supports Redis / In-Memory caching to prevent heavy database queries.
func ListAgents(c *fiber.Ctx) error {
	claims, _ := c.Locals("user").(*middleware.UserClaims)

	masterIDStr := "all"
	if !claims.IsSuperMaster() {
		if claims != nil && claims.MasterAgentID != nil {
			masterIDStr = fmt.Sprintf("%d", *claims.MasterAgentID)
		} else {
			return c.JSON(fiber.Map{
				"success": true,
				"data":    []AgentWithConfigResponse{},
			})
		}
	}

	statusParam := strings.ToLower(strings.TrimSpace(c.Query("status")))
	forceRefresh := c.Query("refresh") == "true"
	cacheKey := fmt.Sprintf("agents_list:%s:%s", masterIDStr, statusParam)

	// Check Redis/In-memory cache if not forcing refresh
	if !forceRefresh {
		if cachedData, found := database.GetReportCache(cacheKey); found {
			var cachedResult []AgentWithConfigResponse
			if err := json.Unmarshal([]byte(cachedData), &cachedResult); err == nil {
				return c.JSON(fiber.Map{
					"success": true,
					"data":    cachedResult,
					"cached":  true,
				})
			}
		}
	}

	query := database.AdminDB.Preload("Connection").Preload("ConnectionRead")

	if !claims.IsSuperMaster() {
		if claims != nil && claims.MasterAgentID != nil {
			query = query.Where("master_agent_id = ?", *claims.MasterAgentID)
		}
	}

	// Filter by is_active according to status parameter
	if statusParam == "active" {
		query = query.Where("is_active = ?", true)
	} else if statusParam == "inactive" {
		query = query.Where("is_active = ?", false)
	} else if statusParam == "maintenance" {
		var maintCodes []string
		_ = database.AdminDB.Model(&models.MasterSiteConfig{}).Where("is_maintenance = ?", true).Pluck("agent_code", &maintCodes).Error
		if len(maintCodes) > 0 {
			query = query.Where("LOWER(agent_code) IN ?", maintCodes)
		} else {
			query = query.Where("1 = 0")
		}
	}

	var agents []models.Agent
	if err := query.Find(&agents).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch agents",
			"error":   err.Error(),
		})
	}

	// Fetch all master site configs
	var siteConfigs []models.MasterSiteConfig
	database.AdminDB.Find(&siteConfigs)

	configMap := make(map[string]models.MasterSiteConfig)
	for _, sc := range siteConfigs {
		configMap[strings.ToLower(sc.AgentCode)] = sc
	}

	result := make([]AgentWithConfigResponse, len(agents))
	for i, ag := range agents {
		sc, exists := configMap[strings.ToLower(ag.AgentCode)]
		var scPtr *models.MasterSiteConfig
		if exists {
			scPtr = &sc
		}
		result[i] = AgentWithConfigResponse{
			Agent:      ag,
			SiteConfig: scPtr,
		}
	}

	// Save to cache (TTL 3 minutes)
	if resultBytes, err := json.Marshal(result); err == nil {
		database.SetReportCache(cacheKey, string(resultBytes), 3*time.Minute)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    result,
		"cached":  false,
	})
}

// GetAgentDetail returns full details for a single agent by UUID
func GetAgentDetail(c *fiber.Ctx) error {
	claims, _ := c.Locals("user").(*middleware.UserClaims)
	agentUUID := c.Params("uuid")

	var agent models.Agent
	if err := database.AdminDB.Preload("Connection").Preload("ConnectionRead").Where("uuid = ?", agentUUID).First(&agent).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Agent not found",
		})
	}

	if !claims.IsSuperMaster() {
		if claims == nil || claims.MasterAgentID == nil || agent.MasterAgentID != *claims.MasterAgentID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "Forbidden: You do not have access to this agent",
			})
		}
	}

	var siteConfig models.MasterSiteConfig
	_ = database.AdminDB.Where("LOWER(agent_code) = ?", strings.ToLower(agent.AgentCode)).First(&siteConfig).Error

	return c.JSON(fiber.Map{
		"success": true,
		"agent":   agent,
		"config":  siteConfig,
	})
}

type UpdateSiteConfigRequest struct {
	ActiveDomain       string `json:"active_domain"`
	BackupDomains      string `json:"backup_domains"`
	IsMaintenance      bool   `json:"is_maintenance"`
	MaintenanceMessage string `json:"maintenance_message"`
	AppName            string `json:"app_name"`
	LogoURL            string `json:"logo_url"`
	ApkDownloadURL     string `json:"apk_download_url"`
}

// UpdateSiteConfig saves or updates master site routing and branding
func UpdateSiteConfig(c *fiber.Ctx) error {
	claims, _ := c.Locals("user").(*middleware.UserClaims)
	agentCode := strings.ToLower(strings.TrimSpace(c.Params("agent_code")))
	if agentCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Agent code is required",
		})
	}

	if err := checkAgentAccess(claims, agentCode); err != nil {
		if fErr, ok := err.(*fiber.Error); ok {
			return c.Status(fErr.Code).JSON(fiber.Map{"success": false, "message": fErr.Message})
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	var req UpdateSiteConfigRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	var siteConfig models.MasterSiteConfig
	err := database.AdminDB.Where("LOWER(agent_code) = ?", agentCode).First(&siteConfig).Error

	siteConfig.AgentCode = agentCode
	siteConfig.ActiveDomain = req.ActiveDomain
	siteConfig.BackupDomains = req.BackupDomains
	siteConfig.IsMaintenance = req.IsMaintenance
	siteConfig.MaintenanceMessage = req.MaintenanceMessage
	siteConfig.AppName = req.AppName
	siteConfig.LogoURL = req.LogoURL
	siteConfig.ApkDownloadURL = req.ApkDownloadURL
	siteConfig.UpdatedAt = time.Now()

	if err != nil {
		if createErr := database.AdminDB.Create(&siteConfig).Error; createErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to create config",
				"error":   createErr.Error(),
			})
		}
	} else {
		if saveErr := database.AdminDB.Save(&siteConfig).Error; saveErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to update config",
				"error":   saveErr.Error(),
			})
		}
	}

	database.InvalidateAgentListCache()

	// Sync tenant MySQL settings.maintenance in background
	go updateTenantSettingsMaintenanceByCode(agentCode, req.IsMaintenance)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Config updated successfully",
		"data":    siteConfig,
	})
}

// flushAgentBackendRedis calls POST /flushRDS on win-backend-api to invalidate cached settings
func flushAgentBackendRedis(apiURL string, secret string) error {
	baseURL := strings.TrimRight(strings.TrimSpace(apiURL), "/")
	if baseURL == "" {
		if config.AppConfig != nil && config.AppConfig.DefaultAgentAPIURL != "" {
			baseURL = config.AppConfig.DefaultAgentAPIURL
		} else {
			baseURL = "http://127.0.0.1:5050" // Fallback default local API
		}
	}
	endpoint := fmt.Sprintf("%s/flushRDS", baseURL)

	req, err := http.NewRequest("POST", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create flush request: %w", err)
	}

	appSecret := strings.TrimSpace(secret)
	if appSecret == "" {
		if config.AppConfig != nil && config.AppConfig.DefaultAgentSecret != "" {
			appSecret = config.AppConfig.DefaultAgentSecret
		} else {
			appSecret = "AbCdEfGh" // Standard app secret from win-backend-api
		}
	}
	req.Header.Set("x-endpoint-secret", appSecret)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("received non-2xx status code %d from %s", resp.StatusCode, endpoint)
	}

	log.Printf("[INFO] win-backend-api Redis flushed successfully via %s\n", endpoint)
	return nil
}

// updateTenantSettingsMaintenance updates the settings table in the tenant's MySQL database
func updateTenantSettingsMaintenance(agentUUID string, isMaintenance bool) error {
	tenantDB, err := database.GetTenantDB(agentUUID, false)
	if err != nil {
		return fmt.Errorf("failed to get tenant DB for %s: %w", agentUUID, err)
	}

	val := 0
	if isMaintenance {
		val = 1
	}

	// Update settings table with timeout to avoid blocking indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := tenantDB.WithContext(ctx).Exec("UPDATE settings SET maintenance = ?", val).Error; err != nil {
		return fmt.Errorf("failed to update settings.maintenance for tenant %s: %w", agentUUID, err)
	}
	log.Printf("[INFO] Updated settings.maintenance = %d for tenant %s\n", val, agentUUID)
	return nil
}

// updateTenantSettingsMaintenanceByCode looks up agent UUID by code, updates settings.maintenance, and flushes win-backend-api Redis
func updateTenantSettingsMaintenanceByCode(agentCode string, isMaintenance bool) {
	var agent models.Agent
	if err := database.AdminDB.Where("LOWER(agent_code) = ?", strings.ToLower(agentCode)).First(&agent).Error; err != nil {
		log.Printf("[WARN] Agent %s not found in Postgres when updating tenant settings: %v\n", agentCode, err)
		return
	}
	if err := updateTenantSettingsMaintenance(agent.UUID, isMaintenance); err != nil {
		log.Printf("[WARN] Failed to sync tenant MySQL maintenance for %s: %v\n", agentCode, err)
	}

	// Flush Redis cache on win-backend-api so the new settings take effect immediately
	apiURL := ""
	if agent.AgentAPIURL != nil {
		apiURL = *agent.AgentAPIURL
	}
	secret := ""
	if agent.AgentSecret != nil {
		secret = *agent.AgentSecret
	}
	if err := flushAgentBackendRedis(apiURL, secret); err != nil {
		log.Printf("[WARN] Failed to flush win-backend-api Redis for %s: %v\n", agentCode, err)
	}
}

// QuickSwitchDomain allows instantaneous domain switching
func QuickSwitchDomain(c *fiber.Ctx) error {
	claims, _ := c.Locals("user").(*middleware.UserClaims)
	agentCode := strings.ToLower(strings.TrimSpace(c.Params("agent_code")))

	if err := checkAgentAccess(claims, agentCode); err != nil {
		if fErr, ok := err.(*fiber.Error); ok {
			return c.Status(fErr.Code).JSON(fiber.Map{"success": false, "message": fErr.Message})
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	type SwitchReq struct {
		Domain string `json:"domain"`
	}
	var req SwitchReq
	if err := c.BodyParser(&req); err != nil || req.Domain == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Valid domain is required",
		})
	}

	var siteConfig models.MasterSiteConfig
	if err := database.AdminDB.Where("LOWER(agent_code) = ?", agentCode).First(&siteConfig).Error; err != nil {
		siteConfig = models.MasterSiteConfig{
			AgentCode:    agentCode,
			ActiveDomain: req.Domain,
			UpdatedAt:    time.Now(),
		}
		database.AdminDB.Create(&siteConfig)
	} else {
		siteConfig.ActiveDomain = req.Domain
		siteConfig.UpdatedAt = time.Now()
		database.AdminDB.Save(&siteConfig)
	}

	database.InvalidateAgentListCache()

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Active domain switched successfully",
		"data":    siteConfig,
	})
}

// QuickToggleMaintenance enables or disables maintenance with 1-click
func QuickToggleMaintenance(c *fiber.Ctx) error {
	claims, _ := c.Locals("user").(*middleware.UserClaims)
	agentCode := strings.ToLower(strings.TrimSpace(c.Params("agent_code")))

	if err := checkAgentAccess(claims, agentCode); err != nil {
		if fErr, ok := err.(*fiber.Error); ok {
			return c.Status(fErr.Code).JSON(fiber.Map{"success": false, "message": fErr.Message})
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	type ToggleReq struct {
		IsMaintenance bool   `json:"is_maintenance"`
		Message       string `json:"message"`
	}
	var req ToggleReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	var siteConfig models.MasterSiteConfig
	if err := database.AdminDB.Where("LOWER(agent_code) = ?", agentCode).First(&siteConfig).Error; err != nil {
		siteConfig = models.MasterSiteConfig{
			AgentCode:          agentCode,
			IsMaintenance:      req.IsMaintenance,
			MaintenanceMessage: req.Message,
			UpdatedAt:          time.Now(),
		}
		database.AdminDB.Create(&siteConfig)
	} else {
		siteConfig.IsMaintenance = req.IsMaintenance
		if req.Message != "" {
			siteConfig.MaintenanceMessage = req.Message
		}
		siteConfig.UpdatedAt = time.Now()
		database.AdminDB.Save(&siteConfig)
	}

	database.InvalidateAgentListCache()

	// Sync tenant MySQL settings.maintenance in background
	go updateTenantSettingsMaintenanceByCode(agentCode, req.IsMaintenance)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Maintenance status updated",
		"data":    siteConfig,
	})
}

// BulkMaintenanceRequest payload for updating all agents maintenance at once
type BulkMaintenanceRequest struct {
	IsMaintenance bool     `json:"is_maintenance"`
	Message       string   `json:"message"`
	AgentCodes    []string `json:"agent_codes,omitempty"` // If empty, applies to all agents within user's scope
}

// BulkMaintenance enables or disables maintenance mode across all agents at once
func BulkMaintenance(c *fiber.Ctx) error {
	claims, _ := c.Locals("user").(*middleware.UserClaims)

	var req BulkMaintenanceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	query := database.AdminDB.Model(&models.Agent{})
	if !claims.IsSuperMaster() {
		if claims != nil && claims.MasterAgentID != nil {
			query = query.Where("master_agent_id = ?", *claims.MasterAgentID)
		} else {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "Forbidden: No master agent assigned",
			})
		}
	}

	if len(req.AgentCodes) > 0 {
		lowerCodes := make([]string, len(req.AgentCodes))
		for i, ac := range req.AgentCodes {
			lowerCodes[i] = strings.ToLower(strings.TrimSpace(ac))
		}
		query = query.Where("LOWER(agent_code) IN ?", lowerCodes)
	}

	var agents []models.Agent
	if err := query.Find(&agents).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch agents",
			"error":   err.Error(),
		})
	}

	if len(agents) == 0 {
		return c.JSON(fiber.Map{
			"success":        true,
			"message":        "Tidak ada agen yang ditemukan untuk diperbarui",
			"affected_count": 0,
		})
	}

	now := time.Now()
	msg := req.Message
	if msg == "" && req.IsMaintenance {
		msg = "Seluruh sistem sedang dalam pemeliharaan server darurat. Silakan coba beberapa saat lagi."
	}

	// Update or insert MasterSiteConfig in a safe transaction
	err := database.AdminDB.Transaction(func(tx *gorm.DB) error {
		for _, ag := range agents {
			code := strings.ToLower(ag.AgentCode)
			var sc models.MasterSiteConfig
			findErr := tx.Where("LOWER(agent_code) = ?", code).First(&sc).Error
			if findErr != nil {
				sc = models.MasterSiteConfig{
					AgentCode:          code,
					AgentName:          ag.Name,
					IsMaintenance:      req.IsMaintenance,
					MaintenanceMessage: msg,
					AppName:            ag.Name,
					UpdatedAt:          now,
				}
				if err := tx.Create(&sc).Error; err != nil {
					return err
				}
			} else {
				sc.IsMaintenance = req.IsMaintenance
				if req.IsMaintenance && msg != "" {
					sc.MaintenanceMessage = msg
				}
				sc.UpdatedAt = now
				if err := tx.Save(&sc).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Gagal menerapkan bulk maintenance",
			"error":   err.Error(),
		})
	}

	database.InvalidateAgentListCache()

	// Sync tenant MySQL settings.maintenance concurrently in background
	go func(targetAgents []models.Agent, isMaint bool) {
		var wg sync.WaitGroup
		sem := make(chan struct{}, 10) // Limit concurrency to 10 workers
		for _, ag := range targetAgents {
			wg.Add(1)
			sem <- struct{}{}
			go func(a models.Agent) {
				defer wg.Done()
				defer func() { <-sem }()
				if err := updateTenantSettingsMaintenance(a.UUID, isMaint); err != nil {
					log.Printf("[WARN] Bulk: Failed to sync tenant MySQL maintenance for %s: %v\n", a.AgentCode, err)
				}
				apiURL := ""
				if a.AgentAPIURL != nil {
					apiURL = *a.AgentAPIURL
				}
				secret := ""
				if a.AgentSecret != nil {
					secret = *a.AgentSecret
				}
				if err := flushAgentBackendRedis(apiURL, secret); err != nil {
					log.Printf("[WARN] Bulk: Failed to flush win-backend-api Redis for %s: %v\n", a.AgentCode, err)
				}
			}(ag)
		}
		wg.Wait()
		log.Printf("[INFO] Bulk maintenance sync to tenant MySQL & Redis completed for %d agents\n", len(targetAgents))
	}(agents, req.IsMaintenance)

	actionWord := "dinonaktifkan (Normal / Online)"
	if req.IsMaintenance {
		actionWord = "diaktifkan (Maintenance Mode)"
	}

	return c.JSON(fiber.Map{
		"success":        true,
		"message":        fmt.Sprintf("Status pemeliharaan berhasil %s untuk %d agen", actionWord, len(agents)),
		"affected_count": len(agents),
		"is_maintenance": req.IsMaintenance,
	})
}

// SyncAgentDomainFromTenant fetches settings.host_url WHERE is_main_domain = 1 from tenant MySQL,
// splits host_url as an array, takes the LAST item, and saves it into master_site_configs.active_domain.
func SyncAgentDomainFromTenant(agentUUID string, agentCode string) (string, error) {
	tenantDB, err := database.GetTenantDB(agentUUID, true)
	if err != nil {
		return "", fmt.Errorf("failed to connect to tenant MySQL database for agent %s: %w", agentCode, err)
	}

	var results []struct {
		HostURL string `gorm:"column:host_url"`
		Web     string `gorm:"column:web"`
		Icon    string `gorm:"column:icon"`
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Query settings WHERE is_main_domain = 1
	err = tenantDB.WithContext(ctx).Table("settings").
		Select("host_url, web, icon").
		Where("is_main_domain = ?", 1).
		Order("id DESC").
		Find(&results).Error

	if err != nil {
		return "", fmt.Errorf("failed to query settings table in tenant DB: %w", err)
	}

	if len(results) == 0 {
		return "", fmt.Errorf("no settings found with is_main_domain = 1 for agent %s", agentCode)
	}

	// Turn host_url into array (split by comma) and take the LAST item
	rawHostURL := results[0].HostURL
	parts := strings.Split(rawHostURL, ",")
	var validDomains []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		trimmed = strings.Trim(trimmed, `"'[] `)
		if trimmed != "" {
			validDomains = append(validDomains, trimmed)
		}
	}

	if len(validDomains) == 0 {
		return "", fmt.Errorf("host_url is empty in settings for agent %s", agentCode)
	}

	lastDomain := validDomains[len(validDomains)-1]
	formattedDomain := lastDomain
	if !strings.HasPrefix(formattedDomain, "http://") && !strings.HasPrefix(formattedDomain, "https://") {
		formattedDomain = "https://" + formattedDomain
	}
	formattedDomain = strings.TrimSuffix(formattedDomain, "/")

	// Extract web (appName) and icon (logoURL) from settings
	tenantAppName := strings.TrimSpace(results[0].Web)
	tenantIcon := strings.TrimSpace(results[0].Icon)
	if tenantIcon != "" && !strings.HasPrefix(tenantIcon, "http://") && !strings.HasPrefix(tenantIcon, "https://") {
		if !strings.HasPrefix(tenantIcon, "/") {
			tenantIcon = "/" + tenantIcon
		}
		tenantIcon = formattedDomain + tenantIcon
	}

	// Save to master_site_configs.active_domain in PostgreSQL
	var siteConfig models.MasterSiteConfig
	dbErr := database.AdminDB.Where("LOWER(agent_code) = ?", strings.ToLower(agentCode)).First(&siteConfig).Error

	siteConfig.AgentCode = strings.ToLower(agentCode)
	siteConfig.ActiveDomain = formattedDomain
	if tenantAppName != "" {
		siteConfig.AppName = tenantAppName
	}
	if tenantIcon != "" {
		siteConfig.LogoURL = tenantIcon
	}
	siteConfig.UpdatedAt = time.Now()

	if dbErr != nil {
		if createErr := database.AdminDB.Create(&siteConfig).Error; createErr != nil {
			return "", fmt.Errorf("failed to create master_site_configs: %w", createErr)
		}
	} else {
		if saveErr := database.AdminDB.Save(&siteConfig).Error; saveErr != nil {
			return "", fmt.Errorf("failed to update master_site_configs: %w", saveErr)
		}
	}

	database.InvalidateAgentListCache()
	return formattedDomain, nil
}

// SyncSingleAgentDomainFromTenant handles POST /api/v1/agents/:agent_code/sync-tenant-domain
func SyncSingleAgentDomainFromTenant(c *fiber.Ctx) error {
	claims, _ := c.Locals("user").(*middleware.UserClaims)
	agentCode := strings.ToLower(strings.TrimSpace(c.Params("agent_code")))
	if agentCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Agent code is required",
		})
	}

	if err := checkAgentAccess(claims, agentCode); err != nil {
		if fErr, ok := err.(*fiber.Error); ok {
			return c.Status(fErr.Code).JSON(fiber.Map{"success": false, "message": fErr.Message})
		}
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": err.Error()})
	}

	var agent models.Agent
	if err := database.AdminDB.Where("LOWER(agent_code) = ?", agentCode).First(&agent).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Agent not found in database",
		})
	}

	newDomain, err := SyncAgentDomainFromTenant(agent.UUID, agent.AgentCode)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success":       true,
		"message":       "Domain aktif berhasil disinkronkan dari MySQL tenant",
		"agent_code":    agent.AgentCode,
		"active_domain": newDomain,
	})
}

// SyncAllAgentDomainsFromTenant handles POST /api/v1/agents/sync-all-tenant-domains
func SyncAllAgentDomainsFromTenant(c *fiber.Ctx) error {
	claims, _ := c.Locals("user").(*middleware.UserClaims)

	query := database.AdminDB.Preload("Connection")
	if !claims.IsSuperMaster() {
		if claims != nil && claims.MasterAgentID != nil {
			query = query.Where("master_agent_id = ?", *claims.MasterAgentID)
		} else {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"message": "Forbidden: No master agent assigned",
			})
		}
	}

	var agents []models.Agent
	if err := query.Find(&agents).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch agents",
			"error":   err.Error(),
		})
	}

	type SyncItemResult struct {
		AgentCode    string `json:"agent_code"`
		ActiveDomain string `json:"active_domain,omitempty"`
		Error        string `json:"error,omitempty"`
		Success      bool   `json:"success"`
	}

	results := make([]SyncItemResult, len(agents))
	successCount := 0

	for i, ag := range agents {
		if ag.Connection == nil {
			results[i] = SyncItemResult{
				AgentCode: ag.AgentCode,
				Success:   false,
				Error:     "No MySQL connection found in agent_connection",
			}
			continue
		}

		domain, err := SyncAgentDomainFromTenant(ag.UUID, ag.AgentCode)
		if err != nil {
			results[i] = SyncItemResult{
				AgentCode: ag.AgentCode,
				Success:   false,
				Error:     err.Error(),
			}
		} else {
			successCount++
			results[i] = SyncItemResult{
				AgentCode:    ag.AgentCode,
				ActiveDomain: domain,
				Success:      true,
			}
		}
	}

	return c.JSON(fiber.Map{
		"success":       true,
		"message":       fmt.Sprintf("Sinkronisasi selesai: %d dari %d agen berhasil diupdate", successCount, len(agents)),
		"total_agents":  len(agents),
		"success_count": successCount,
		"results":       results,
	})
}
