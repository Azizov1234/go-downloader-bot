package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"instagram-downloader-bot/internal/cache"
	"instagram-downloader-bot/internal/config"
	"instagram-downloader-bot/internal/db"
	"instagram-downloader-bot/internal/donate"
	"instagram-downloader-bot/internal/handlers"
	"instagram-downloader-bot/internal/instagram"
	"instagram-downloader-bot/internal/logs"
	"instagram-downloader-bot/internal/media"
	"instagram-downloader-bot/internal/queue"
	"instagram-downloader-bot/internal/saved"
	"instagram-downloader-bot/internal/settings"
	"instagram-downloader-bot/internal/stats"
	"instagram-downloader-bot/internal/storage"
	"instagram-downloader-bot/internal/telegram"
	"instagram-downloader-bot/internal/users"
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
	adminService := users.NewAdminService(pool)
	if err := adminService.EnsureSuperAdmin(ctx, cfg.SuperAdminTelegramID); err != nil {
		log.Fatal(err)
	}
	userService := users.NewService(pool)
	mediaService := media.NewService(pool)
	variantService := media.NewVariantService(mediaService)
	cacheService := cache.NewFileIDCache(variantService)
	queueClient, err := queue.NewClient(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer queueClient.Close()
	inspector, _ := queue.NewInspector(cfg)
	statsService := stats.NewService(pool, inspector, cfg)
	errorLogs := logs.NewErrorLogService(pool)
	adminLogs := logs.NewAdminActionLogService(pool)
	savedService := saved.NewService(pool)
	donateService := donate.NewService(settingsService)

	bot, err := telegram.New(cfg.BotToken, logg)
	if err != nil {
		log.Fatal(err)
	}
	delivery := media.NewDeliveryService(bot.API)
	cleanup := storage.NewCleanupService(cfg.TempDownloadDir, cfg.TempFilesTTL, cfg.CleanupInterval, logg)
	go cleanup.Start(ctx)

	router := handlers.NewRouter(handlers.Dependencies{
		Bot: bot.API, Config: cfg, Logger: logg, Redis: redisClient, Provider: instagram.NewProvider(),
		Cache: cacheService, Media: mediaService, Users: userService, Admins: adminService,
		Settings: settingsService, Queue: queueClient, Delivery: delivery, Saved: savedService,
		Donate: donateService, Logs: errorLogs, Stats: statsService, AdminLogs: adminLogs,
	})
	bot.Start(ctx, router)
}
