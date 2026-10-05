package handlers

import (
	"strings"

	"master-panel-api/database"
	"master-panel-api/models"

	"github.com/gofiber/fiber/v2"
)

// GetWorkerAgentConfig is a sub-5ms endpoint consumed by Cloudflare Worker and Android APK
func GetWorkerAgentConfig(c *fiber.Ctx) error {
	agentCode := strings.ToLower(strings.TrimSpace(c.Query("agent", "")))
	if agentCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Missing 'agent' query parameter",
		})
	}

	var siteConfig models.MasterSiteConfig
	var appName string = "Win Gaming"
	var logoURL string = ""
	var activeDomain string = ""
	var backupDomains []string = []string{}
	var isMaintenance bool = false
	var maintenanceMsg string = "Kami sedang melakukan pemeliharaan rutin. Silakan coba kembali nanti."
	var apkURL string = "/app/master-release.apk"

	if database.AdminDB != nil {
		err := database.AdminDB.Where("LOWER(agent_code) = ?", agentCode).First(&siteConfig).Error
		if err == nil {
			appName = siteConfig.AppName
			logoURL = siteConfig.LogoURL
			activeDomain = siteConfig.ActiveDomain
			if siteConfig.BackupDomains != "" {
				for _, d := range strings.Split(siteConfig.BackupDomains, ",") {
					cleanD := strings.TrimSpace(d)
					if cleanD != "" {
						backupDomains = append(backupDomains, cleanD)
					}
				}
			}
			isMaintenance = siteConfig.IsMaintenance
			if siteConfig.MaintenanceMessage != "" {
				maintenanceMsg = siteConfig.MaintenanceMessage
			}
			if siteConfig.ApkDownloadURL != "" {
				apkURL = siteConfig.ApkDownloadURL
			}
		} else {
			// Fallback: check if agent exists in PostgreSQL agent table
			var agent models.Agent
			if agErr := database.AdminDB.Where("LOWER(agent_code) = ?", agentCode).First(&agent).Error; agErr == nil {
				appName = agent.Name
			}
		}
	}

	responseData := fiber.Map{
		"agent":               agentCode,
		"agent_code":          agentCode,
		"app_name":            appName,
		"active_url":          activeDomain,
		"active_domain":       activeDomain,
		"backup_domains":      backupDomains,
		"maintenance":         isMaintenance,
		"is_maintenance":      isMaintenance,
		"maintenance_message": maintenanceMsg,
		"logo_url":            logoURL,
		"apk_download_url":    apkURL,
	}

	return c.JSON(fiber.Map{
		"success":             true,
		"status":              "success",
		"agent_code":          agentCode,
		"agent":               agentCode,
		"app_name":            appName,
		"active_url":          activeDomain,
		"active_domain":       activeDomain,
		"backup_domains":      backupDomains,
		"maintenance":         isMaintenance,
		"is_maintenance":      isMaintenance,
		"maintenance_message": maintenanceMsg,
		"logo_url":            logoURL,
		"apk_download_url":    apkURL,
		"timestamp":           c.Context().Time().Unix(),
		"data":                responseData,
	})
}
