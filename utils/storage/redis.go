package storage

import (
	"context"
	"os"

	"github.com/Sojil8/eCommerce-silver/pkg"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var RedisClient *redis.Client
var Ctx = context.Background()

func InitRedis() {
	pkg.Log.Debug("Initializing Redis client")

	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
		pkg.Log.Warn("REDIS_ADDR not set, using default",
			zap.String("addr", addr))
	}

	password := os.Getenv("REDIS_PASSWORD")
	db := 0
	protocol := 2

	RedisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
		Protocol: protocol,
	})

	_, err := RedisClient.Ping(Ctx).Result()
	if err != nil {
		pkg.Log.Fatal("Failed to connect to Redis",
			zap.String("addr", addr),
			zap.Int("db", db),
			zap.Int("protocol", protocol),
			zap.Error(err))
	}

	pkg.Log.Info("Redis client initialized successfully",
		zap.String("addr", addr),
		zap.Int("db", db),
		zap.Int("protocol", protocol))
}