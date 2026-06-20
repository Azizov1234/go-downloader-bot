package config

import (
	"strings"
	"time"

	"github.com/joho/godotenv"

	"instagram-downloader-bot/pkg/utils"
)

type Config struct {
	BotToken                        string
	TelegramAPIEndpoint             string
	TelegramAPIMode                 string
	TelegramCloudAPIURL             string
	TelegramLocalAPIURL             string
	TelegramCloudMaxUploadMB        int64
	TelegramLocalMaxUploadMB        int64
	TelegramUseLocalFilePath        bool
	RequireLocalBotAPIForLargeFiles bool
	DatabaseURL                     string
	RedisURL                        string
	SuperAdminTelegramID            int64
	AppPort                         int
	Env                             string
	OnlyInstagram                   bool
	MaxVideoFileSizeMB              int64
	MaxAudioFileSizeMB              int64
	TelegramMaxUploadMB             int64
	AllowOversizedDownloads         bool
	UploadsDir                      string
	TempDownloadDir                 string
	CacheDir                        string
	TempFilesTTL                    time.Duration
	CleanupInterval                 time.Duration
	YTDLPBin                        string
	FFmpegBin                       string
	FFprobeBin                      string
	InstagramUseCookies             bool
	InstagramCookiesFile            string
	InstagramFormats                map[string]string
	QueuePrefix                     string
	DownloadConcurrency             int
	SendConcurrency                 int
	AudioConcurrency                int
	JobAttempts                     int
	QueueBackoff                    time.Duration
	InFlightLockTTL                 time.Duration
	RateLimitTTL                    time.Duration
	RateLimitLimit                  int
	DailyUserDownloadLimit          int
	BotOnline                       bool
	MaintenanceMode                 bool
	DonateCardNumber                string
	DonateCardOwner                 string
	DonateText                      string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		BotToken:                        utils.StringEnv("BOT_TOKEN", ""),
		TelegramAPIEndpoint:             utils.StringEnv("TELEGRAM_API_ENDPOINT", ""),
		TelegramAPIMode:                 normalizeTelegramMode(utils.StringEnv("TELEGRAM_API_MODE", "cloud")),
		TelegramCloudAPIURL:             utils.StringEnv("TELEGRAM_CLOUD_API_URL", "https://api.telegram.org"),
		TelegramLocalAPIURL:             utils.StringEnv("TELEGRAM_LOCAL_API_URL", "http://127.0.0.1:8081"),
		TelegramCloudMaxUploadMB:        utils.Int64Env("TELEGRAM_CLOUD_MAX_UPLOAD_MB", 50),
		TelegramLocalMaxUploadMB:        utils.Int64Env("TELEGRAM_LOCAL_MAX_UPLOAD_MB", 2000),
		TelegramUseLocalFilePath:        utils.BoolEnv("TELEGRAM_USE_LOCAL_FILE_PATH", true),
		RequireLocalBotAPIForLargeFiles: utils.BoolEnv("REQUIRE_LOCAL_BOT_API_FOR_LARGE_FILES", true),
		DatabaseURL:                     utils.StringEnv("DATABASE_URL", "postgresql://postgres:postgres@localhost:5432/instagram_downloader?sslmode=disable"),
		RedisURL:                        utils.StringEnv("REDIS_URL", "redis://localhost:6379"),
		SuperAdminTelegramID:            utils.Int64Env("SUPERADMIN_TELEGRAM_ID", 0),
		AppPort:                         utils.IntEnv("APP_PORT", 8000),
		Env:                             utils.StringEnv("NODE_ENV", "development"),
		OnlyInstagram:                   utils.BoolEnv("ONLY_INSTAGRAM", true),
		MaxVideoFileSizeMB:              utils.Int64Env("MAX_VIDEO_FILE_SIZE_MB", 500),
		MaxAudioFileSizeMB:              utils.Int64Env("MAX_AUDIO_FILE_SIZE_MB", 100),
		TelegramMaxUploadMB:             utils.Int64Env("TELEGRAM_MAX_UPLOAD_MB", 2000),
		AllowOversizedDownloads:         utils.BoolEnv("ALLOW_OVERSIZED_DOWNLOADS", false),
		UploadsDir:                      utils.StringEnv("UPLOADS_DIR", "uploads"),
		TempDownloadDir:                 utils.StringEnv("TEMP_DOWNLOAD_DIR", "uploads/temp/downloads"),
		CacheDir:                        utils.StringEnv("CACHE_DIR", "uploads/cache"),
		TempFilesTTL:                    time.Duration(utils.IntEnv("TEMP_FILES_TTL_MINUTES", 30)) * time.Minute,
		CleanupInterval:                 time.Duration(utils.IntEnv("CLEANUP_INTERVAL_MINUTES", 10)) * time.Minute,
		YTDLPBin:                        utils.StringEnv("YTDLP_BIN", "yt-dlp"),
		FFmpegBin:                       utils.StringEnv("FFMPEG_BIN", "ffmpeg"),
		FFprobeBin:                      utils.StringEnv("FFPROBE_BIN", "ffprobe"),
		InstagramUseCookies:             utils.BoolEnv("INSTAGRAM_USE_COOKIES", true),
		InstagramCookiesFile:            utils.StringEnv("INSTAGRAM_COOKIES_FILE", "storage/cookies/instagram.cookies.txt"),
		QueuePrefix:                     utils.StringEnv("QUEUE_PREFIX", "instagram_bot"),
		DownloadConcurrency:             utils.IntEnv("INSTAGRAM_DOWNLOAD_QUEUE_CONCURRENCY", 5),
		SendConcurrency:                 utils.IntEnv("INSTAGRAM_SEND_QUEUE_CONCURRENCY", 20),
		AudioConcurrency:                utils.IntEnv("AUDIO_QUEUE_CONCURRENCY", 2),
		JobAttempts:                     utils.IntEnv("QUEUE_JOB_ATTEMPTS", 2),
		QueueBackoff:                    utils.DurationMsEnv("QUEUE_BACKOFF_MS", 5*time.Second),
		InFlightLockTTL:                 time.Duration(utils.IntEnv("IN_FLIGHT_LOCK_TTL_SECONDS", 180)) * time.Second,
		RateLimitTTL:                    time.Duration(utils.IntEnv("RATE_LIMIT_TTL", 60)) * time.Second,
		RateLimitLimit:                  utils.IntEnv("RATE_LIMIT_LIMIT", 30),
		DailyUserDownloadLimit:          utils.IntEnv("DAILY_USER_DOWNLOAD_LIMIT", 100),
		BotOnline:                       utils.BoolEnv("BOT_ONLINE", true),
		MaintenanceMode:                 utils.BoolEnv("MAINTENANCE_MODE", false),
		DonateCardNumber:                utils.StringEnv("DONATE_CARD_NUMBER", "8600000000000000"),
		DonateCardOwner:                 utils.StringEnv("DONATE_CARD_OWNER", "YOUR NAME"),
		DonateText:                      utils.StringEnv("DONATE_TEXT", "Donat uchun karta raqami:"),
	}
	cfg.InstagramFormats = map[string]string{
		"AUTO":     utils.StringEnv("INSTAGRAM_FORMAT_AUTO", "best[ext=mp4]/best"),
		"ORIGINAL": utils.StringEnv("INSTAGRAM_FORMAT_ORIGINAL", "bestvideo[ext=mp4]+bestaudio[ext=m4a]/best[ext=mp4]/best"),
		"P1080":    utils.StringEnv("INSTAGRAM_FORMAT_1080", "bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/best[height<=1080][ext=mp4]/best"),
		"P720":     utils.StringEnv("INSTAGRAM_FORMAT_720", "bestvideo[height<=720][ext=mp4]+bestaudio[ext=m4a]/best[height<=720][ext=mp4]/best"),
		"P480":     utils.StringEnv("INSTAGRAM_FORMAT_480", "bestvideo[height<=480][ext=mp4]+bestaudio[ext=m4a]/best[height<=480][ext=mp4]/best"),
		"SMALL":    utils.StringEnv("INSTAGRAM_FORMAT_SMALL", "worst[ext=mp4]/worst"),
	}
	return cfg, nil
}

func (c Config) TelegramEndpointForMode(mode string) string {
	if c.TelegramAPIEndpoint != "" {
		return c.TelegramAPIEndpoint
	}
	if normalizeTelegramMode(mode) == "local" {
		return telegramEndpoint(c.TelegramLocalAPIURL)
	}
	return telegramEndpoint(c.TelegramCloudAPIURL)
}

func (c Config) TelegramStartupEndpoint() string {
	return c.TelegramEndpointForMode(c.TelegramAPIMode)
}

func normalizeTelegramMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "local" {
		return "local"
	}
	return "cloud"
}

func telegramEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.telegram.org"
	}
	if strings.Contains(baseURL, "%s") {
		return baseURL
	}
	return baseURL + "/bot%s/%s"
}
