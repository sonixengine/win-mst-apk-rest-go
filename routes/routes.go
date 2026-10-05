package routes

import (
	"master-panel-api/handlers"
	"master-panel-api/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func SetupRoutes(app *fiber.App) {
	// Middleware global
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "*",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-Master-Secret",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
		AllowCredentials: false,
	}))

	// Health Check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"service": "master-panel-api",
		})
	})

	api := app.Group("/api/v1")

	// 1. Gateway Route (Cloudflare Worker & Mobile APK)
	gateway := api.Group("/gateway")
	gateway.Get("/config", middleware.RequireWorkerSecret(), handlers.GetWorkerAgentConfig)
	api.Get("/agent-config", middleware.RequireWorkerSecret(), handlers.GetWorkerAgentConfig)

	// 2. Auth Routes
	auth := api.Group("/auth")
	auth.Post("/login", handlers.Login)

	// 3. Protected Master Panel Routes
	protected := api.Group("", middleware.Protected())

	// Current User
	protected.Get("/auth/me", handlers.Me)

	// Agents Management
	agents := protected.Group("/agents")
	agents.Get("/", handlers.ListAgents)
	agents.Post("/bulk-maintenance", handlers.BulkMaintenance)
	agents.Get("/:uuid", handlers.GetAgentDetail)
	agents.Post("/sync-all-tenant-domains", handlers.SyncAllAgentDomainsFromTenant)
	agents.Post("/:agent_code/sync-tenant-domain", handlers.SyncSingleAgentDomainFromTenant)
	agents.Post("/:agent_code/config", handlers.UpdateSiteConfig)
	agents.Post("/:agent_code/switch-domain", handlers.QuickSwitchDomain)
	agents.Post("/:agent_code/maintenance", handlers.QuickToggleMaintenance)

	// Tenant Player Database (MySQL per agent)
	agents.Get("/:uuid/player-summary", handlers.GetAgentPlayerSummary)
	agents.Get("/:uuid/players", handlers.ListAgentPlayers)

	// Winlose Reports (Protected with RequireWinloseAccess - blocked for legacy engine_auth and restricted operators)
	reports := protected.Group("/reports", middleware.RequireWinloseAccess())
	reports.Get("/agents", handlers.ListReportAgents)
	reports.Get("/winlose/agent/:uuid", handlers.GetAgentWinloseReport)
	reports.Get("/winlose/agent/:uuid/players", handlers.GetAgentActivePlayersReport)

	// User & Role Management (SQLite Local DB - Superadmin and Administrator only)
	users := protected.Group("/users", middleware.RequireUserManagement())
	users.Get("/", handlers.ListLocalUsers)
	users.Post("/", handlers.CreateLocalUser)
	users.Put("/:id", handlers.UpdateLocalUser)
	users.Delete("/:id", handlers.DeleteLocalUser)

	roles := protected.Group("/roles", middleware.RequireUserManagement())
	roles.Get("/", handlers.ListLocalRoles)
	roles.Post("/", handlers.CreateLocalRole)
	roles.Put("/:id", handlers.UpdateLocalRole)
}
