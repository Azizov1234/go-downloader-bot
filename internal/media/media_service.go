package media

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DownloadRecord struct {
	ID int64
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) GetOrCreateMediaFile(ctx context.Context, originalURL, normalizedURL, shortcode string) (MediaFile, error) {
	var m MediaFile
	err := s.db.QueryRow(ctx, `
		INSERT INTO media_files (original_url, normalized_url, instagram_shortcode, platform)
		VALUES ($1, $2, $3, 'instagram')
		ON CONFLICT (normalized_url) DO UPDATE
		SET original_url=EXCLUDED.original_url, instagram_shortcode=EXCLUDED.instagram_shortcode, updated_at=now()
		RETURNING id, original_url, normalized_url, COALESCE(instagram_shortcode,''), platform
	`, originalURL, normalizedURL, shortcode).Scan(&m.ID, &m.OriginalURL, &m.NormalizedURL, &m.InstagramShortcode, &m.Platform)
	return m, err
}

func (s *Service) FindCachedVariant(ctx context.Context, normalizedURL string, variantType VariantType, quality Quality) (MediaVariant, error) {
	return s.findVariant(ctx, normalizedURL, variantType, quality, true)
}

func (s *Service) FindVariant(ctx context.Context, normalizedURL string, variantType VariantType, quality Quality) (MediaVariant, error) {
	return s.findVariant(ctx, normalizedURL, variantType, quality, false)
}

func (s *Service) findVariant(ctx context.Context, normalizedURL string, variantType VariantType, quality Quality, requireFileID bool) (MediaVariant, error) {
	query := `
		SELECT id, media_file_id, normalized_url, original_url, COALESCE(instagram_shortcode,''), variant_type,
		       quality, COALESCE(telegram_file_id,''), COALESCE(telegram_file_unique_id,''), width, height,
		       duration, fps, codec, file_size, status, created_at, updated_at
		FROM media_variants
		WHERE normalized_url=$1 AND variant_type=$2 AND quality=$3`
	if requireFileID {
		query += ` AND telegram_file_id IS NOT NULL AND telegram_file_id <> ''`
	}
	return scanVariant(s.db.QueryRow(ctx, query, normalizedURL, string(variantType), string(quality)))
}

func (s *Service) UpsertVariant(ctx context.Context, mediaFile MediaFile, variantType VariantType, quality Quality, fileID, uniqueID string, md Metadata, status string) (MediaVariant, error) {
	return scanVariant(s.db.QueryRow(ctx, `
		INSERT INTO media_variants (
			media_file_id, normalized_url, original_url, instagram_shortcode, variant_type, quality,
			telegram_file_id, telegram_file_unique_id, width, height, duration, fps, codec, file_size, status
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (normalized_url, variant_type, quality) DO UPDATE
		SET telegram_file_id=EXCLUDED.telegram_file_id,
		    telegram_file_unique_id=EXCLUDED.telegram_file_unique_id,
		    width=EXCLUDED.width,
		    height=EXCLUDED.height,
		    duration=EXCLUDED.duration,
		    fps=EXCLUDED.fps,
		    codec=EXCLUDED.codec,
		    file_size=EXCLUDED.file_size,
		    status=EXCLUDED.status,
		    updated_at=now()
		RETURNING id, media_file_id, normalized_url, original_url, COALESCE(instagram_shortcode,''), variant_type,
		       quality, COALESCE(telegram_file_id,''), COALESCE(telegram_file_unique_id,''), width, height,
		       duration, fps, codec, file_size, status, created_at, updated_at
	`, mediaFile.ID, mediaFile.NormalizedURL, mediaFile.OriginalURL, mediaFile.InstagramShortcode, string(variantType), string(quality),
		nullableString(fileID), nullableString(uniqueID), md.Width, md.Height, md.Duration, md.FPS, md.Codec, md.FileSize, status))
}

func (s *Service) ClearFileID(ctx context.Context, variantID int64) error {
	_, err := s.db.Exec(ctx, `
		UPDATE media_variants
		SET telegram_file_id=NULL, telegram_file_unique_id=NULL, status='STALE', updated_at=now()
		WHERE id=$1
	`, variantID)
	return err
}

func (s *Service) GetVariantByID(ctx context.Context, id int64) (MediaVariant, error) {
	return scanVariant(s.db.QueryRow(ctx, `
		SELECT id, media_file_id, normalized_url, original_url, COALESCE(instagram_shortcode,''), variant_type,
		       quality, COALESCE(telegram_file_id,''), COALESCE(telegram_file_unique_id,''), width, height,
		       duration, fps, codec, file_size, status, created_at, updated_at
		FROM media_variants WHERE id=$1
	`, id))
}

func scanVariant(row pgx.Row) (MediaVariant, error) {
	var v MediaVariant
	var variantType, quality string
	err := row.Scan(
		&v.ID, &v.MediaFileID, &v.NormalizedURL, &v.OriginalURL, &v.InstagramShortcode, &variantType,
		&quality, &v.TelegramFileID, &v.TelegramFileUniqueID, &v.Width, &v.Height, &v.Duration,
		&v.FPS, &v.Codec, &v.FileSize, &v.Status, &v.CreatedAt, &v.UpdatedAt,
	)
	v.VariantType = VariantType(variantType)
	v.Quality = Quality(quality)
	return v, err
}

func (s *Service) CreateDownload(ctx context.Context, userID int64, variantID *int64, status string, cacheHit bool, cacheLookup time.Duration) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `
		INSERT INTO downloads (user_id, media_variant_id, status, cache_hit, cache_lookup_ms)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, userID, variantID, status, cacheHit, cacheLookup.Milliseconds()).Scan(&id)
	return id, err
}

func (s *Service) UpdateDownloadMetrics(ctx context.Context, id int64, variantID *int64, status string, queueWait, download, convert, send, total time.Duration, errMessage string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE downloads
		SET media_variant_id=$2, status=$3, queue_wait_ms=$4, download_ms=$5, convert_ms=$6,
		    send_ms=$7, total_ms=$8, error_message=$9
		WHERE id=$1
	`, id, variantID, status, queueWait.Milliseconds(), download.Milliseconds(), convert.Milliseconds(), send.Milliseconds(), total.Milliseconds(), nullableString(errMessage))
	return err
}

func (s *Service) MarkDaily(ctx context.Context, variantType VariantType, cacheHit bool, status string, oversized bool) {
	_, _ = s.db.Exec(ctx, `
		INSERT INTO daily_stats (date, downloads_count, success_count, failed_count, cache_hit_count, cache_miss_count, audio_count, video_count, oversized_rejected_count)
		VALUES (CURRENT_DATE, 1, $1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (date) DO UPDATE
		SET downloads_count=daily_stats.downloads_count+1,
		    success_count=daily_stats.success_count+EXCLUDED.success_count,
		    failed_count=daily_stats.failed_count+EXCLUDED.failed_count,
		    cache_hit_count=daily_stats.cache_hit_count+EXCLUDED.cache_hit_count,
		    cache_miss_count=daily_stats.cache_miss_count+EXCLUDED.cache_miss_count,
		    audio_count=daily_stats.audio_count+EXCLUDED.audio_count,
		    video_count=daily_stats.video_count+EXCLUDED.video_count,
		    oversized_rejected_count=daily_stats.oversized_rejected_count+EXCLUDED.oversized_rejected_count,
		    updated_at=now()
	`, boolToInt(status == "SUCCESS"), boolToInt(status == "FAILED"), boolToInt(cacheHit), boolToInt(!cacheHit),
		boolToInt(variantType == VariantAudio), boolToInt(variantType == VariantVideo), boolToInt(oversized))
}

func nullableString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func IsNoRows(err error) bool {
	return err == pgx.ErrNoRows
}
