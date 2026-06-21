package stats

import (
	"context"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"

	"instagram-downloader-bot/internal/config"
)

type Service struct {
	db        *pgxpool.Pool
	inspector *asynq.Inspector
	cfg       config.Config
}

func NewService(db *pgxpool.Pool, inspector *asynq.Inspector, cfg config.Config) *Service {
	return &Service{db: db, inspector: inspector, cfg: cfg}
}

func (s *Service) Summary(ctx context.Context) (string, error) {
	var users, downloads, success, failed, cacheHit, cacheMiss, audio, video, oversized int64
	_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&users)
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*),
		       COUNT(*) FILTER (WHERE status='SUCCESS'),
		       COUNT(*) FILTER (WHERE status='FAILED'),
		       COUNT(*) FILTER (WHERE cache_hit),
		       COUNT(*) FILTER (WHERE NOT cache_hit)
		FROM downloads
	`).Scan(&downloads, &success, &failed, &cacheHit, &cacheMiss)
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE variant_type='AUDIO'), COUNT(*) FILTER (WHERE variant_type='VIDEO')
		FROM media_variants
	`).Scan(&audio, &video)
	_ = s.db.QueryRow(ctx, `SELECT COALESCE(SUM(oversized_rejected_count),0) FROM daily_stats`).Scan(&oversized)

	var todayUsers, todayDownloads, todaySuccess, activeUsers24h int64
	_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE created_at::date = CURRENT_DATE`).Scan(&todayUsers)
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE status='SUCCESS')
		FROM downloads WHERE created_at::date = CURRENT_DATE
	`).Scan(&todayDownloads, &todaySuccess)
	_ = s.db.QueryRow(ctx, `SELECT COUNT(DISTINCT user_id) FROM downloads WHERE created_at >= now() - interval '24 hours'`).Scan(&activeUsers24h)

	var avgDownload, avgSend, avgTotal float64
	_ = s.db.QueryRow(ctx, `SELECT COALESCE(AVG(download_ms),0), COALESCE(AVG(send_ms),0), COALESCE(AVG(total_ms),0) FROM downloads`).Scan(&avgDownload, &avgSend, &avgTotal)

	queueLines := s.queueStats()
	quality, _ := s.QualityStats(ctx)
	return fmt.Sprintf(
		"📊 Statistika\n\n" +
			"👥 Jami foydalanuvchilar: %d\n" +
			"✨ Bugun qo'shilganlar: %d\n" +
			"🔥 Faol userlar (oxirgi 24s): %d\n\n" +
			"📥 Jami yuklashlar: %d\n" +
			"✅ Muvaffaqiyatli: %d\n" +
			"❌ Xatolar: %d\n" +
			"⚡ Kesh orqali (Hit): %d\n" +
			"🔄 Keshsiz (Miss): %d\n" +
			"📹 Video yuklamalar: %d\n" +
			"🎵 Audio yuklamalar: %d\n" +
			"⚠️ Limitdan oshganlar: %d\n\n" +
			"📅 Bugungi yuklashlar: %d\n" +
			"📅 Bugungi muvaffaqiyatli: %d\n\n" +
			"⏱️ O'rtacha yuklash vaqti: %.0f ms\n" +
			"⏱️ O'rtacha yuborish vaqti: %.0f ms\n" +
			"⏱️ Jami o'rtacha vaqt: %.0f ms\n\n" +
			"Quality:\n%s\n\n" +
			"Queue:\n%s",
		users, todayUsers, activeUsers24h,
		downloads, success, failed, cacheHit, cacheMiss, video, audio, oversized,
		todayDownloads, todaySuccess,
		avgDownload, avgSend, avgTotal, quality, queueLines,
	), nil
}

func (s *Service) QualityStats(ctx context.Context) (string, error) {
	rows, err := s.db.Query(ctx, `SELECT quality, COUNT(*) FROM media_variants GROUP BY quality ORDER BY quality`)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var q string
		var count int64
		if err := rows.Scan(&q, &count); err != nil {
			return "", err
		}
		lines = append(lines, fmt.Sprintf("%s: %d", q, count))
	}
	if len(lines) == 0 {
		return "hali ma'lumot yo'q", nil
	}
	return strings.Join(lines, "\n"), rows.Err()
}

func (s *Service) queueStats() string {
	if s.inspector == nil {
		return "inspector ulanmagan"
	}
	queues := []string{"instagram_download", "instagram_send", "instagram_audio_convert", "cleanup", "notification"}
	var lines []string
	for _, q := range queues {
		info, err := s.inspector.GetQueueInfo(q)
		if err != nil {
			lines = append(lines, q+": n/a")
			continue
		}
		lines = append(lines, fmt.Sprintf("%s waiting=%d active=%d failed=%d", q, info.Pending, info.Active, info.Archived+info.Retry))
	}
	return strings.Join(lines, "\n")
}
