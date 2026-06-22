package handlers

import (
	"context"
	"log/slog"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"

	"instagram-downloader-bot/internal/cache"
	"instagram-downloader-bot/internal/config"
	"instagram-downloader-bot/internal/donate"
	"instagram-downloader-bot/internal/handlers/admin"
	"instagram-downloader-bot/internal/instagram"
	"instagram-downloader-bot/internal/logs"
	"instagram-downloader-bot/internal/media"
	"instagram-downloader-bot/internal/queue"
	"instagram-downloader-bot/internal/saved"
	"instagram-downloader-bot/internal/settings"
	"instagram-downloader-bot/internal/stats"
	"instagram-downloader-bot/internal/users"
)

type Router struct {
	bot      *tgbotapi.BotAPI
	cfg      config.Config
	logger   *slog.Logger
	redis    *redis.Client
	provider instagram.Provider
	cache    *cache.FileIDCache
	media    *media.Service
	users    *users.Service
	admins   *users.AdminService
	settings *settings.Service
	queue    *queue.Client
	delivery *media.DeliveryService
	saved    *saved.Service
	donate   *donate.Service
	logs     *logs.ErrorLogService
	admin    *admin.Handler
}

type Dependencies struct {
	Bot       *tgbotapi.BotAPI
	Config    config.Config
	Logger    *slog.Logger
	Redis     *redis.Client
	Provider  instagram.Provider
	Cache     *cache.FileIDCache
	Media     *media.Service
	Users     *users.Service
	Admins    *users.AdminService
	Settings  *settings.Service
	Queue     *queue.Client
	Delivery  *media.DeliveryService
	Saved     *saved.Service
	Donate    *donate.Service
	Logs      *logs.ErrorLogService
	Stats     *stats.Service
	AdminLogs *logs.AdminActionLogService
}

func NewRouter(dep Dependencies) *Router {
	adminHandler := admin.NewHandler(admin.Dependencies{
		Bot: dep.Bot, Redis: dep.Redis, Settings: dep.Settings, Admins: dep.Admins,
		Users: dep.Users, Stats: dep.Stats, Logs: dep.Logs, AdminLogs: dep.AdminLogs, Delivery: dep.Delivery,
	})
	return &Router{
		bot: dep.Bot, cfg: dep.Config, logger: dep.Logger, redis: dep.Redis,
		provider: dep.Provider, cache: dep.Cache, media: dep.Media, users: dep.Users, admins: dep.Admins,
		settings: dep.Settings, queue: dep.Queue, delivery: dep.Delivery, saved: dep.Saved,
		donate: dep.Donate, logs: dep.Logs, admin: adminHandler,
	}
}

func (r *Router) HandleUpdate(ctx context.Context, update tgbotapi.Update) {
	defer func() {
		if rec := recover(); rec != nil {
			r.logger.Error("panic in update handler", "recover", rec)
		}
	}()
	if update.CallbackQuery != nil {
		r.handleCallback(ctx, update.CallbackQuery)
		return
	}
	if update.Message != nil {
		r.handleMessage(ctx, update.Message)
	}
}

func (r *Router) send(chatID int64, text string, replyMarkup any) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = replyMarkup
	_, _ = r.bot.Send(msg)
}

func (r *Router) edit(chatID int64, messageID int, text string, replyMarkup any) {
	msg := tgbotapi.NewEditMessageText(chatID, messageID, text)
	if markup, ok := replyMarkup.(tgbotapi.InlineKeyboardMarkup); ok {
		msg.ReplyMarkup = &markup
	}
	_, _ = r.bot.Send(msg)
}

func (r *Router) answerCallback(id, text string) {
	cfg := tgbotapi.NewCallback(id, text)
	_, _ = r.bot.Request(cfg)
}

func (r *Router) rateLimited(ctx context.Context, telegramID int64) bool {
	key := "rate:" + strconvFormat(telegramID)
	n, err := r.redis.Incr(ctx, key).Result()
	if err == nil && n == 1 {
		_ = r.redis.Expire(ctx, key, r.cfg.RateLimitTTL).Err()
	}
	return err == nil && int(n) > r.cfg.RateLimitLimit
}

func strconvFormat(v int64) string {
	return strings.TrimSpace(strconv.FormatInt(v, 10))
}
