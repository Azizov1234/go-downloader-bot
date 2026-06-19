package cache

import (
	"context"
	"time"

	"instagram-downloader-bot/internal/media"
)

type LookupResult struct {
	Variant media.MediaVariant
	Hit     bool
	Took    time.Duration
}

type FileIDCache struct {
	variants *media.VariantService
}

func NewFileIDCache(variants *media.VariantService) *FileIDCache {
	return &FileIDCache{variants: variants}
}

func (c *FileIDCache) Lookup(ctx context.Context, normalizedURL string, variantType media.VariantType, quality media.Quality) LookupResult {
	start := time.Now()
	variant, err := c.variants.Cached(ctx, normalizedURL, variantType, quality)
	return LookupResult{Variant: variant, Hit: err == nil && variant.TelegramFileID != "", Took: time.Since(start)}
}
