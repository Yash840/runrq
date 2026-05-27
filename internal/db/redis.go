package db

import (
	"github.com/Yash840/runrq/internal/config"
	"github.com/redis/go-redis/v9"
)

func NewRedisClient(cfg *config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RAddr,
		Password: cfg.RPassword,
	})
}
