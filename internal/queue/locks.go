package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"

	"instagram-downloader-bot/internal/media"
)

type Locks struct {
	redis *redis.Client
	ttl   time.Duration
}

func NewLocks(redisClient *redis.Client, ttl time.Duration) *Locks {
	return &Locks{redis: redisClient, ttl: ttl}
}

func LockKey(normalizedURL string, variantType media.VariantType, quality media.Quality) string {
	return media.CacheKey(normalizedURL, variantType, quality) + ":lock"
}

func WaitersKey(normalizedURL string, variantType media.VariantType, quality media.Quality) string {
	return media.CacheKey(normalizedURL, variantType, quality) + ":waiters"
}

func (l *Locks) Acquire(ctx context.Context, key string) (bool, error) {
	return l.redis.SetNX(ctx, key, "1", l.ttl).Result()
}

func (l *Locks) Release(ctx context.Context, key string) {
	_ = l.redis.Del(ctx, key).Err()
}

func (l *Locks) AddWaiter(ctx context.Context, key string, r Recipient) error {
	body, err := json.Marshal(r)
	if err != nil {
		return err
	}
	pipe := l.redis.TxPipeline()
	pipe.RPush(ctx, key, body)
	pipe.Expire(ctx, key, l.ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (l *Locks) PopWaiters(ctx context.Context, key string) ([]Recipient, error) {
	raw, err := l.redis.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	_ = l.redis.Del(ctx, key).Err()
	out := make([]Recipient, 0, len(raw))
	seen := map[int64]bool{}
	for _, item := range raw {
		var r Recipient
		if err := json.Unmarshal([]byte(item), &r); err != nil {
			continue
		}
		if seen[r.ChatID] {
			continue
		}
		seen[r.ChatID] = true
		out = append(out, r)
	}
	return out, nil
}
