package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"

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

type selectionState struct {
	OriginalURL        string            `json:"original_url"`
	NormalizedURL      string            `json:"normalized_url"`
	InstagramShortcode string            `json:"instagram_shortcode"`
	VariantType        media.VariantType `json:"variant_type"`
	Quality            media.Quality     `json:"quality"`
	UserID             int64             `json:"user_id"`
	ChatID             int64             `json:"chat_id"`
	CustomTitle        string            `json:"custom_title,omitempty"`
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

func (r *Router) setSelection(ctx context.Context, st selectionState) (string, error) {
	token := randomToken()
	body, err := json.Marshal(st)
	if err != nil {
		return "", err
	}
	err = r.redis.Set(ctx, "selection:"+token, body, 15*time.Minute).Err()
	return token, err
}

func (r *Router) getSelection(ctx context.Context, token string) (selectionState, error) {
	raw, err := r.redis.Get(ctx, "selection:"+token).Bytes()
	if err != nil {
		return selectionState{}, err
	}
	var st selectionState
	if err := json.Unmarshal(raw, &st); err != nil {
		return selectionState{}, err
	}
	return st, nil
}

func (r *Router) saveSelection(ctx context.Context, token string, st selectionState) error {
	body, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return r.redis.Set(ctx, "selection:"+token, body, 15*time.Minute).Err()
}

func (r *Router) deleteSelection(ctx context.Context, token string) {
	_ = r.redis.Del(ctx, "selection:"+token).Err()
}

func (r *Router) setPendingRename(ctx context.Context, telegramID int64, token string) {
	_ = r.redis.Set(ctx, "pending_rename:"+strconvFormat(telegramID), token, 5*time.Minute).Err()
}

func randomToken() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func strconvFormat(v int64) string {
	return strings.TrimSpace(strconv.FormatInt(v, 10))
}
