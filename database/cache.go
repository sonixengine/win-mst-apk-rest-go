package database

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"master-panel-api/config"

	"github.com/redis/go-redis/v9"
)

var (
	RedisClient *redis.Client
	redisOnce   sync.Once
	isRedisUp   bool

	// In-memory fallback cache if Redis is not configured or offline
	memCache      sync.Map
	memCacheMutex sync.RWMutex
)

type memCacheItem struct {
	value     string
	expiresAt time.Time
}

// InitCache initializes Redis connection with fallback to in-memory cache
func InitCache() {
	redisOnce.Do(func() {
		if config.AppConfig == nil || !config.AppConfig.RedisEnabled {
			log.Println("[INFO] Redis is disabled in config, using thread-safe in-memory cache")
			return
		}

		dbNum, _ := strconv.Atoi(config.AppConfig.RedisDB)
		rdb := redis.NewClient(&redis.Options{
			Addr:        fmt.Sprintf("%s:%s", config.AppConfig.RedisHost, config.AppConfig.RedisPort),
			Password:    config.AppConfig.RedisPass,
			DB:          dbNum,
			DialTimeout: 2 * time.Second,
			ReadTimeout: 3 * time.Second,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := rdb.Ping(ctx).Err(); err != nil {
			log.Printf("[WARN] Redis ping failed (%s:%s): %v. Falling back to in-memory cache\n",
				config.AppConfig.RedisHost, config.AppConfig.RedisPort, err)
			isRedisUp = false
		} else {
			log.Printf("[INFO] Connected to Redis at %s:%s (DB: %d)\n",
				config.AppConfig.RedisHost, config.AppConfig.RedisPort, dbNum)
			RedisClient = rdb
			isRedisUp = true
		}
	})
}

// IsRedisAvailable returns true if active Redis connection exists
func IsRedisAvailable() bool {
	return isRedisUp && RedisClient != nil
}

// GetReportCache fetches cached report string by key
func GetReportCache(key string) (string, bool) {
	if IsRedisAvailable() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		val, err := RedisClient.Get(ctx, key).Result()
		if err == nil && val != "" {
			return val, true
		}
		if err != redis.Nil {
			// Redis connection blip, fallback to memory
			log.Printf("[DEBUG] Redis GET error for key %s: %v\n", key, err)
		}
	}

	// In-memory cache fallback
	if itemRaw, ok := memCache.Load(key); ok {
		item := itemRaw.(memCacheItem)
		if time.Now().Before(item.expiresAt) {
			return item.value, true
		}
		// Expired item cleanup
		memCache.Delete(key)
	}

	return "", false
}

// SetReportCache stores report JSON in Redis or in-memory cache with specified TTL
func SetReportCache(key string, val string, ttl time.Duration) {
	if IsRedisAvailable() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := RedisClient.Set(ctx, key, val, ttl).Err(); err != nil {
			log.Printf("[WARN] Failed to set Redis cache for %s: %v\n", key, err)
		}
	}

	// Always update in-memory cache as resilient backup
	memCache.Store(key, memCacheItem{
		value:     val,
		expiresAt: time.Now().Add(ttl),
	})
}

// InvalidateReportCache removes a specific key from both caches (used for force refresh)
func InvalidateReportCache(key string) {
	if IsRedisAvailable() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		RedisClient.Del(ctx, key)
	}
	memCache.Delete(key)
}

// InvalidateAgentListCache clears all cached agent listings matching prefix agents_list:
func InvalidateAgentListCache() {
	if IsRedisAvailable() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		iter := RedisClient.Scan(ctx, 0, "agents_list:*", 0).Iterator()
		for iter.Next(ctx) {
			RedisClient.Del(ctx, iter.Val())
		}
	}
	memCache.Range(func(key, value interface{}) bool {
		if kStr, ok := key.(string); ok && strings.HasPrefix(kStr, "agents_list:") {
			memCache.Delete(key)
		}
		return true
	})
}
