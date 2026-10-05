package database

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"master-panel-api/config"
	"master-panel-api/models"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var LocalDB *gorm.DB

// InitLocalDB initializes the SQLite database for local user and role management
func InitLocalDB(cfg *config.Config) error {
	dbPath := cfg.SqliteDBPath
	if dbPath == "" {
		dbPath = "data/master_panel.db"
	}

	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory for sqlite db: %w", err)
		}
	}

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
	LocalDB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: customLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to open local sqlite database at %s: %w", dbPath, err)
	}

	sqlDB, err := LocalDB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB from sqlite: %w", err)
	}
	// SQLite performs best with 1 open writer or small pool
	sqlDB.SetMaxOpenConns(1)

	log.Printf("[INFO] Successfully connected to local SQLite database at: %s\n", dbPath)

	// Run auto migration
	if err := LocalDB.AutoMigrate(&models.LocalRole{}, &models.LocalUser{}); err != nil {
		return fmt.Errorf("failed to auto-migrate local sqlite tables: %w", err)
	}

	// Seed roles and initial superadmin on first install
	if err := seedInitialRolesAndUser(cfg); err != nil {
		log.Printf("[WARN] Error during initial sqlite seeding: %v\n", err)
	}

	return nil
}

func seedInitialRolesAndUser(cfg *config.Config) error {
	godRoleName := cfg.GodRoleName
	if godRoleName == "" {
		godRoleName = "GOD Admin"
	}

	// 1. Seed standard roles if not present
	roles := []models.LocalRole{
		{
			Code:             "superadmin",
			Name:             godRoleName,
			Description:      "Akses penuh (Level GOD): seluruh fitur aktif dan akun tersembunyi dari publik.",
			CanManageUsers:   true,
			CanManageRoles:   true,
			CanViewWinlose:   true,
			CanManageDomains: true,
			CanUseSimulator:  true,
		},
		{
			Code:             "admin",
			Name:             "Administrator",
			Description:      "Akses manajemen agen, ubah domain, laporan winlose, gateway simulator, dan manajemen user.",
			CanManageUsers:   true,
			CanManageRoles:   true,
			CanViewWinlose:   true,
			CanManageDomains: true,
			CanUseSimulator:  true,
		},
		{
			Code:             "operator",
			Name:             "Operator",
			Description:      "Akses terbatas untuk monitoring agen dan simulator tanpa izin melihat winlose atau mengelola user.",
			CanManageUsers:   false,
			CanManageRoles:   false,
			CanViewWinlose:   false,
			CanManageDomains: false,
			CanUseSimulator:  true,
		},
	}

	for _, r := range roles {
		var existing models.LocalRole
		if err := LocalDB.Where("code = ?", r.Code).First(&existing).Error; err != nil {
			if err := LocalDB.Create(&r).Error; err != nil {
				log.Printf("[WARN] Failed to seed role %s: %v\n", r.Code, err)
			} else {
				log.Printf("[INFO] Seeded default role: %s (%s)\n", r.Name, r.Code)
			}
		} else if r.Code == "superadmin" && existing.Name != godRoleName {
			// Update nama role superadmin jika diset di ENV (misal: "GOD Admin")
			LocalDB.Model(&existing).Update("name", godRoleName)
		}
	}

	// 2. Ensure GOD Admin account always exists and credentials stay in sync with ENV
	godEmail := strings.ToLower(strings.TrimSpace(cfg.InitialSuperadminEmail))
	if godEmail == "" {
		godEmail = "superadmin@masterpanel.local"
	}
	godPass := cfg.InitialSuperadminPass
	if godPass == "" {
		godPass = "admin123456"
	}
	godName := cfg.InitialSuperadminName
	if godName == "" {
		godName = "GOD Administrator"
	}

	var superRole models.LocalRole
	if err := LocalDB.Where("code = ?", "superadmin").First(&superRole).Error; err != nil {
		return fmt.Errorf("superadmin role not found for seeding user: %w", err)
	}

	var godUser models.LocalUser
	errGod := LocalDB.Where("LOWER(email) = ?", godEmail).First(&godUser).Error

	if errGod != nil {
		// Belum ada user dengan email GOD admin ini, buat baru
		hashed, err := bcrypt.GenerateFromPassword([]byte(godPass), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash default admin password: %w", err)
		}

		initialUser := models.LocalUser{
			UUID:      uuid.New().String(),
			Name:      godName,
			Email:     godEmail,
			Password:  string(hashed),
			RoleID:    superRole.ID,
			IsActive:  true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := LocalDB.Create(&initialUser).Error; err != nil {
			return fmt.Errorf("failed to create initial god admin user: %w", err)
		}

		log.Println("=================================================================")
		log.Printf("[INIT] %s created:\n", godRoleName)
		log.Printf("       Name    : %s\n", godName)
		log.Printf("       Email   : %s\n", godEmail)
		log.Printf("       Password: %s\n", godPass)
		log.Printf("       Role    : %s\n", godRoleName)
		log.Println("=================================================================")
	} else {
		// Jika sudah ada, pastikan selalu aktif, rolenya superadmin, dan sinkronkan password dari ENV jika berubah
		needsUpdate := false
		if !godUser.IsActive {
			godUser.IsActive = true
			needsUpdate = true
		}
		if godUser.RoleID != superRole.ID {
			godUser.RoleID = superRole.ID
			needsUpdate = true
		}
		if godUser.Name != godName && godName != "" {
			godUser.Name = godName
			needsUpdate = true
		}
		// Cek apakah password di DB cocok dengan godPass dari ENV
		if err := bcrypt.CompareHashAndPassword([]byte(godUser.Password), []byte(godPass)); err != nil {
			newHashed, errHash := bcrypt.GenerateFromPassword([]byte(godPass), bcrypt.DefaultCost)
			if errHash == nil {
				godUser.Password = string(newHashed)
				needsUpdate = true
				log.Printf("[INFO] %s password synced from environment variable.\n", godRoleName)
			}
		}

		if needsUpdate {
			godUser.UpdatedAt = time.Now()
			_ = LocalDB.Save(&godUser)
		}
	}

	return nil
}
