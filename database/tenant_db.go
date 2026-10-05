package database

import (
	"fmt"
	"log"
	"sync"
	"time"

	"master-panel-api/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	tenantPools sync.Map // map[string]*gorm.DB
	poolMutex   sync.Mutex
)

// GetTenantDB retrieves or dynamically establishes a connection to an agent's MySQL player database.
func GetTenantDB(agentUUID string, readOnly bool) (*gorm.DB, error) {
	cacheKey := fmt.Sprintf("%s_ro:%t", agentUUID, readOnly)

	// 1. Check existing connection in cache
	if dbVal, ok := tenantPools.Load(cacheKey); ok {
		if tenantGorm, ok := dbVal.(*gorm.DB); ok {
			// Verify if connection is still healthy
			if sqlDB, err := tenantGorm.DB(); err == nil {
				if err := sqlDB.Ping(); err == nil {
					return tenantGorm, nil
				}
			}
		}
	}

	// 2. Lock to prevent race condition while establishing new connection
	poolMutex.Lock()
	defer poolMutex.Unlock()

	// Double check after acquiring lock
	if dbVal, ok := tenantPools.Load(cacheKey); ok {
		return dbVal.(*gorm.DB), nil
	}

	// 3. Fetch credentials from PostgreSQL Admin DB
	var host, dbName, user, password string
	var port int

	if readOnly {
		var connRead models.AgentConnectionRead
		err := AdminDB.Where("agent_uuid = ?", agentUUID).First(&connRead).Error
		if err != nil {
			log.Printf("[INFO] Read-only DB connection not found for agent %s, falling back to primary\n", agentUUID)
			readOnly = false
		} else {
			host = connRead.DBHost
			dbName = connRead.DBName
			user = connRead.DBUser
			password = connRead.DBPassword
			port = connRead.DBPort
		}
	}

	if !readOnly {
		var conn models.AgentConnection
		if err := AdminDB.Where("agent_uuid = ?", agentUUID).First(&conn).Error; err != nil {
			return nil, fmt.Errorf("no database connection found in agent_connection for agent %s: %w", agentUUID, err)
		}
		host = conn.DBHost
		dbName = conn.DBName
		user = conn.DBUser
		password = conn.DBPassword
		port = conn.DBPort
	}

	if port == 0 {
		port = 3306
	}

	// 4. Construct MySQL DSN
	// [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s&readTimeout=10s",
		user, password, host, port, dbName,
	)

	tenantGorm, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL player DB for agent %s: %w", agentUUID, err)
	}

	sqlDB, err := tenantGorm.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB for tenant: %w", err)
	}

	// Configure pool parameters
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetConnMaxLifetime(10 * time.Minute)

	// Save to thread-safe cache
	tenantPools.Store(cacheKey, tenantGorm)
	log.Printf("[INFO] Dynamic MySQL pool created for agent %s (%s:%d/%s)\n", agentUUID, host, port, dbName)

	return tenantGorm, nil
}
