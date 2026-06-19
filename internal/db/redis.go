package db

import (
	"context"

	"github.com/redis/go-redis/v9"

	"instagram-downloader-bot/internal/config"
)

func NewRedis(ctx context.Context, cfg config.Config) (*redis.Client, error) {
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}
