package main

import (
	"log"

	"master-panel-api/config"
	"master-panel-api/database"
	"master-panel-api/routes"

	"github.com/gofiber/fiber/v2"
)

func main() {
	cfg := config.LoadConfig()

	if err := database.InitAdminDB(cfg); err != nil {
		log.Fatalf("[FATAL] Database connection failed: %v", err)
	}

	if err := database.InitLocalDB(cfg); err != nil {
		log.Fatalf("[FATAL] SQLite LocalDB initialization failed: %v", err)
	}

	database.InitCache()

	app := fiber.New(fiber.Config{
		AppName:      "Master Panel API v1.0",
		ServerHeader: "Go-Fiber",
	})

	routes.SetupRoutes(app)

	addr := ":" + cfg.AppPort
	log.Printf("[INFO] Master Panel API running on http://0.0.0.0%s\n", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("[FATAL] Server failed to start: %v", err)
	}
}
