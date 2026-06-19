package settings

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"instagram-downloader-bot/internal/config"
)

type Settings struct {
	BotOnline            bool
	MaintenanceMode       bool
	MaxVideoFileSizeMB    int64
	MaxAudioFileSizeMB    int64
	TelegramMaxUploadMB   int64
	WelcomeText           string
	HelpText              string
	DonateCardNumber      string
	DonateCardOwner       string
	DonateText            string
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) EnsureDefaults(ctx context.Context, cfg config.Config) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO bot_settings (
			id, bot_online, maintenance_mode, max_video_file_size_mb, max_audio_file_size_mb,
			telegram_max_upload_mb, welcome_text, help_text, donate_card_number, donate_card_owner, donate_text
		)
		VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO NOTHING
	`, cfg.BotOnline, cfg.MaintenanceMode, cfg.MaxVideoFileSizeMB, cfg.MaxAudioFileSizeMB,
		cfg.TelegramMaxUploadMB, "Instagram havolasini yuboring.", "Public yoki ruxsatli Instagram link yuboring.",
		cfg.DonateCardNumber, cfg.DonateCardOwner, cfg.DonateText)
	return err
}

func (s *Service) Get(ctx context.Context) (Settings, error) {
	var out Settings
	err := s.db.QueryRow(ctx, `
		SELECT bot_online, maintenance_mode, max_video_file_size_mb, max_audio_file_size_mb,
		       telegram_max_upload_mb, welcome_text, help_text, donate_card_number, donate_card_owner, donate_text
		FROM bot_settings
		ORDER BY id
		LIMIT 1
	`).Scan(&out.BotOnline, &out.MaintenanceMode, &out.MaxVideoFileSizeMB, &out.MaxAudioFileSizeMB,
		&out.TelegramMaxUploadMB, &out.WelcomeText, &out.HelpText, &out.DonateCardNumber, &out.DonateCardOwner, &out.DonateText)
	if err == pgx.ErrNoRows {
		return Settings{}, fmt.Errorf("bot_settings row not found")
	}
	return out, err
}

func (s *Service) SetBool(ctx context.Context, column string, value bool) error {
	if column != "bot_online" && column != "maintenance_mode" {
		return fmt.Errorf("unsupported bool setting: %s", column)
	}
	_, err := s.db.Exec(ctx, fmt.Sprintf("UPDATE bot_settings SET %s=$1, updated_at=now() WHERE id=1", column), value)
	return err
}

func (s *Service) SetInt64(ctx context.Context, column string, value int64) error {
	if column != "max_video_file_size_mb" && column != "max_audio_file_size_mb" && column != "telegram_max_upload_mb" {
		return fmt.Errorf("unsupported int setting: %s", column)
	}
	_, err := s.db.Exec(ctx, fmt.Sprintf("UPDATE bot_settings SET %s=$1, updated_at=now() WHERE id=1", column), value)
	return err
}

func (s *Service) SetText(ctx context.Context, column, value string) error {
	switch column {
	case "welcome_text", "help_text", "donate_card_number", "donate_card_owner", "donate_text":
	default:
		return fmt.Errorf("unsupported text setting: %s", column)
	}
	_, err := s.db.Exec(ctx, fmt.Sprintf("UPDATE bot_settings SET %s=$1, updated_at=now() WHERE id=1", column), value)
	return err
}
