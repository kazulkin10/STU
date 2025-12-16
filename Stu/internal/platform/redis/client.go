package redis

import (
	"github.com/redis/go-redis/v9"

	"stu/internal/config"
)

// New creates a Redis client from config.
func New(cfg config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}
