package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort        string
	JWTSecret      string
	WorkerSecret   string
	AdminDBHost    string
	AdminDBPort    string
	AdminDBUser    string
	AdminDBPass    string
	AdminDBName    string
	AdminDBSSLMode string
	RedisHost      string
	RedisPort      string
	RedisPass      string
	RedisDB        string
	RedisEnabled   bool

	// Default fallback connection for win-backend-api
	DefaultAgentAPIURL string
	DefaultAgentSecret string
	// SQLite Local DB for local users & role management
	SqliteDBPath           string
	InitialSuperadminEmail string
	InitialSuperadminPass  string
	InitialSuperadminName  string
	GodRoleName            string
}

var AppConfig *Config

func LoadConfig() *Config {
	// Try loading .env if exists
	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] No .env file found or using system environment variables")
	}

	rawSSL := strings.ToLower(strings.TrimSpace(getEnv("ADMIN_DB_SSLMODE", "disable")))
	sslMode := "disable"
	switch rawSSL {
	case "enable", "enabled", "require", "required", "true", "1", "yes":
		sslMode = "require"
	case "verify-ca":
		sslMode = "verify-ca"
	case "verify-full":
		sslMode = "verify-full"
	case "prefer":
		sslMode = "prefer"
	case "allow":
		sslMode = "allow"
	default:
		sslMode = "disable"
	}

	redisEnabledStr := strings.ToLower(strings.TrimSpace(getEnv("REDIS_ENABLED", "true")))
	redisEnabled := redisEnabledStr == "true" || redisEnabledStr == "1" || redisEnabledStr == "yes"

	AppConfig = &Config{
		AppPort:                getEnv("APP_PORT", "8080"),
		JWTSecret:              getEnv("JWT_SECRET", "super-secret-master-panel-jwt-key"),
		WorkerSecret:           getEnv("WORKER_API_SECRET", "hrc-worker-secret-key-2026"),
		AdminDBHost:            getEnv("ADMIN_DB_HOST", "62.72.46.195"),
		AdminDBPort:            getEnv("ADMIN_DB_PORT", "2022"),
		AdminDBUser:            getEnv("ADMIN_DB_USER", "postgres"),
		AdminDBPass:            getEnv("ADMIN_DB_PASS", "postgres"),
		AdminDBName:            getEnv("ADMIN_DB_NAME", "engine-auth"),
		AdminDBSSLMode:         sslMode,
		RedisHost:              getEnv("REDIS_HOST", "127.0.0.1"),
		RedisPort:              getEnv("REDIS_PORT", "6379"),
		RedisPass:              getEnv("REDIS_PASSWORD", ""),
		RedisDB:                getEnv("REDIS_DB", "0"),
		RedisEnabled:           redisEnabled,
		DefaultAgentAPIURL:     getEnv("DEFAULT_AGENT_API_URL", "http://127.0.0.1:5050"),
		DefaultAgentSecret:     getEnv("DEFAULT_AGENT_SECRET", "AbCdEfGh"),
		SqliteDBPath:           getEnv("SQLITE_DB_PATH", "data/master_panel.db"),
		InitialSuperadminEmail: getEnv("INITIAL_SUPERADMIN_EMAIL", "superadmin@masterpanel.local"),
		InitialSuperadminPass:  getEnv("INITIAL_SUPERADMIN_PASSWORD", "admin123456"),
		InitialSuperadminName:  getEnv("INITIAL_SUPERADMIN_NAME", "GOD Administrator"),
		GodRoleName:            getEnv("GOD_ROLE_NAME", "GOD Admin"),
	}

	return AppConfig
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
