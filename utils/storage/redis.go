package storage

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
)

// redis start = systemctl start redis
var RedisClient *redis.Client
var Ctx = context.Background()

func InitRedis() {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "your-redis-password",
		DB:       0,
		Protocol: 2, 
	})

	ctx := context.Background()
	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Failed to connect to Redis")
	}
}
