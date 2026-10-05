package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"master-panel-api/config"
	"master-panel-api/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var AdminDB *gorm.DB

// InitAdminDB initializes the primary connection to the PostgreSQL Admin DB (engine-auth)
func InitAdminDB(cfg *config.Config) error {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=Asia/Jakarta",
		cfg.AdminDBHost,
		cfg.AdminDBUser,
		cfg.AdminDBPass,
		cfg.AdminDBName,
		cfg.AdminDBPort,
		cfg.AdminDBSSLMode,
	)
	customLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	var err error
	AdminDB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: customLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to Admin PostgreSQL database: %w", err)
	}

	sqlDB, err := AdminDB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Println("[INFO] Successfully connected to Admin PostgreSQL database.")

	// Auto-migrate our custom master panel config table (safe: does not alter existing engine-auth tables)
	if err := AdminDB.AutoMigrate(&models.MasterSiteConfig{}); err != nil {
		log.Printf("[WARN] MasterSiteConfig auto-migration notice: %v\n", err)
	}

	return nil
}
