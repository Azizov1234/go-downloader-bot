package saved

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"instagram-downloader-bot/internal/media"
)

type Item struct {
	ID             int64
	SaveNumber     int64
	VariantID      int64
	TelegramFileID string
	Platform       string
	Quality        media.Quality
	VariantType    media.VariantType
	OriginalURL    string
	NormalizedURL  string
	CreatedAt      time.Time
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) Save(ctx context.Context, userID int64, variant media.MediaVariant) (Item, error) {
	var item Item
	var quality, variantType string
	err := s.db.QueryRow(ctx, `
		INSERT INTO saved_media (
			save_number, user_id, media_variant_id, telegram_file_id, platform, quality, variant_type, original_url, normalized_url
		)
		VALUES (nextval('saved_media_number_seq'), $1, $2, $3, 'instagram', $4, $5, $6, $7)
		ON CONFLICT (user_id, media_variant_id) DO UPDATE
		SET telegram_file_id=EXCLUDED.telegram_file_id
		RETURNING id, save_number, media_variant_id, telegram_file_id, platform, quality, variant_type, original_url, normalized_url, created_at
	`, userID, variant.ID, variant.TelegramFileID, string(variant.Quality), string(variant.VariantType), variant.OriginalURL, variant.NormalizedURL).
		Scan(&item.ID, &item.SaveNumber, &item.VariantID, &item.TelegramFileID, &item.Platform, &quality, &variantType, &item.OriginalURL, &item.NormalizedURL, &item.CreatedAt)
	item.Quality = media.Quality(quality)
	item.VariantType = media.VariantType(variantType)
	return item, err
}

func (s *Service) List(ctx context.Context, userID int64, page, perPage int) ([]Item, int, error) {
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * perPage
	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM saved_media WHERE user_id=$1`, userID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, save_number, media_variant_id, telegram_file_id, platform, quality, variant_type, original_url, normalized_url, created_at
		FROM saved_media
		WHERE user_id=$1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var item Item
		var quality, variantType string
		if err := rows.Scan(&item.ID, &item.SaveNumber, &item.VariantID, &item.TelegramFileID, &item.Platform, &quality, &variantType, &item.OriginalURL, &item.NormalizedURL, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		item.Quality = media.Quality(quality)
		item.VariantType = media.VariantType(variantType)
		out = append(out, item)
	}
	return out, total, rows.Err()
}

func Number(n int64) string {
	return fmt.Sprintf("#%06d", n)
}
