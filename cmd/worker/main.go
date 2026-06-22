package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"instagram-downloader-bot/internal/config"
	"instagram-downloader-bot/internal/db"
	"instagram-downloader-bot/internal/downloader"
	"instagram-downloader-bot/internal/instagram"
	"instagram-downloader-bot/internal/logs"
	"instagram-downloader-bot/internal/media"
	"instagram-downloader-bot/internal/queue"
	"instagram-downloader-bot/internal/settings"
	"instagram-downloader-bot/internal/storage"
	"instagram-downloader-bot/internal/telegram"
	"instagram-downloader-bot/internal/users"
	"instagram-downloader-bot/internal/workers"
	"instagram-downloader-bot/pkg/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	logg := logger.New(cfg.Env)
	if cfg.BotToken == "" {
		log.Fatal("BOT_TOKEN is required")
	}
	if err := db.RunMigrations(cfg.DatabaseURL, "internal/db/migrations"); err != nil {
		log.Fatal(err)
	}
	pool, err := db.NewPostgres(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	redisClient, err := db.NewRedis(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer redisClient.Close()

	settingsService := settings.NewService(pool)
	if err := settingsService.EnsureDefaults(ctx, cfg); err != nil {
		log.Fatal(err)
	}
	st, err := settingsService.Get(ctx)
	if err != nil {
		logg.Error("failed to get settings for logging", "error", err)
	} else {
		effectiveLimit := st.TelegramCloudMaxUploadMB
		if st.TelegramAPIMode == "local" {
			effectiveLimit = st.TelegramLocalMaxUploadMB
		}
		logg.Info("active telegram api mode",
			"telegram_api_mode", st.TelegramAPIMode,
			"local_max_mb", st.TelegramLocalMaxUploadMB,
			"cloud_max_mb", st.TelegramCloudMaxUploadMB,
			"effective_upload_limit_mb", effectiveLimit,
			"local_api_url", cfg.TelegramLocalAPIURL,
		)
		log.Printf("active telegram api mode=%s effective_upload_limit_mb=%d local_api_url=%s",
			st.TelegramAPIMode, effectiveLimit, cfg.TelegramLocalAPIURL)
	}
	adminService := users.NewAdminService(pool)
	if err := adminService.EnsureSuperAdmin(ctx, cfg.SuperAdminTelegramID); err != nil {
		log.Fatal(err)
	}
	userService := users.NewService(pool)
	mediaService := media.NewService(pool)
	errorLogs := logs.NewErrorLogService(pool)
	queueClient, err := queue.NewClient(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer queueClient.Close()
	locks := queue.NewLocks(redisClient, cfg.InFlightLockTTL)
	storageService := storage.NewService(cfg.TempDownloadDir)
	cleanup := storage.NewCleanupService(cfg.TempDownloadDir, cfg.TempFilesTTL, cfg.CleanupInterval, logg)
	go cleanup.Start(ctx)

	bot, err := telegram.New(cfg.BotToken, cfg.TelegramStartupEndpoint(), logg)
	if err != nil && cfg.TelegramAPIMode == "local" {
		logg.Warn("local telegram api unavailable at startup, falling back to cloud polling", "error", err)
		bot, err = telegram.New(cfg.BotToken, cfg.TelegramEndpointForMode("cloud"), logg)
	}
	if err != nil {
		log.Fatal(err)
	}
	delivery := media.NewDeliveryService(bot.API, cfg, settingsService)
	ytdlpEngine := downloader.YTDLP{Bin: cfg.YTDLPBin}
	galleryDLEngine := downloader.GalleryDL{Bin: cfg.GalleryDLBin}
	fallbackDownloader := downloader.NewFallbackDownloader(ytdlpEngine, galleryDLEngine, logg)

	downloadWorker := workers.NewDownloadWorker(workers.DownloadWorkerDeps{
		Bot: bot.API, Logger: logg,
		Downloader: fallbackDownloader,
		RichProber: fallbackDownloader,
		FFProbe:    downloader.FFProbe{Bin: cfg.FFprobeBin},
		FFMpeg:     downloader.FFMpeg{Bin: cfg.FFmpegBin},
		Formats:    instagram.NewFormatBuilder(cfg.InstagramFormats),
		Cookies:    instagram.Cookies{Use: cfg.InstagramUseCookies, File: cfg.InstagramCookiesFile},
		Storage:    storageService, Media: mediaService, Settings: settingsService,
		Users:      userService, Logs: errorLogs, Queue: queueClient, Locks: locks,
		AllowOversized: cfg.AllowOversizedDownloads,
	})
	audioWorker := workers.NewAudioWorker(bot.API, downloader.FFMpeg{Bin: cfg.FFmpegBin}, storageService, settingsService, mediaService, queueClient, locks, cfg.AllowOversizedDownloads)
	sendWorker := workers.NewSendWorker(bot.API, delivery, mediaService, userService, queueClient, storageService, locks, errorLogs)
	cleanupWorker := workers.NewCleanupWorker(cleanup)
	notificationWorker := workers.NewNotificationWorker(bot.API, cfg)

	redisOpt, err := queue.RedisOpt(cfg.RedisURL)
	if err != nil {
		log.Fatal(err)
	}
	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: cfg.DownloadConcurrency + cfg.SendConcurrency + cfg.AudioConcurrency + 2,
		Queues: map[string]int{
			queue.QueueDownload:     cfg.DownloadConcurrency,
			queue.QueueSend:         cfg.SendConcurrency,
			queue.QueueAudioConvert: cfg.AudioConcurrency,
			queue.QueueCleanup:      1,
			queue.QueueNotification: 1,
		},
		RetryDelayFunc: func(n int, err error, task *asynq.Task) time.Duration {
			return cfg.QueueBackoff * time.Duration(n)
		},
	})
	mux := asynq.NewServeMux()
	mux.Handle(queue.TypeDownload, downloadWorker)
	mux.Handle(queue.TypeAudioConvert, audioWorker)
	mux.Handle(queue.TypeSend, sendWorker)
	mux.Handle(queue.TypeCleanup, cleanupWorker)
	mux.Handle(queue.TypeNotification, notificationWorker)

	errCh := make(chan error, 1)
	go func() { errCh <- server.Run(mux) }()
	logg.Info("worker started")
	select {
	case <-ctx.Done():
		server.Shutdown()
	case err := <-errCh:
		if err != nil {
			log.Fatal(err)
		}
	}
}
