package admin

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"

	"instagram-downloader-bot/internal/logs"
	"instagram-downloader-bot/internal/media"
	"instagram-downloader-bot/internal/settings"
	"instagram-downloader-bot/internal/stats"
	"instagram-downloader-bot/internal/telegram"
	"instagram-downloader-bot/internal/users"
)

type Handler struct {
	bot       *tgbotapi.BotAPI
	redis     *redis.Client
	settings  *settings.Service
	admins    *users.AdminService
	stats     *stats.Service
	logs      *logs.ErrorLogService
	adminLogs *logs.AdminActionLogService
	delivery  *media.DeliveryService
}

type Dependencies struct {
	Bot       *tgbotapi.BotAPI
	Redis     *redis.Client
	Settings  *settings.Service
	Admins    *users.AdminService
	Stats     *stats.Service
	Logs      *logs.ErrorLogService
	AdminLogs *logs.AdminActionLogService
	Delivery  *media.DeliveryService
}

func NewHandler(dep Dependencies) *Handler {
	return &Handler{bot: dep.Bot, redis: dep.Redis, settings: dep.Settings, admins: dep.Admins, stats: dep.Stats, logs: dep.Logs, adminLogs: dep.AdminLogs, delivery: dep.Delivery}
}

func (h *Handler) Show(ctx context.Context, chatID, telegramID int64) {
	ok, _, err := h.admins.IsAdmin(ctx, telegramID)
	if err != nil || !ok {
		h.send(chatID, "Admin panelga ruxsat yo'q.", nil)
		return
	}
	h.send(chatID, "Admin panel", telegram.AdminKeyboard())
}

func (h *Handler) HandleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	ok, adminUser, err := h.admins.IsAdmin(ctx, cb.From.ID)
	if err != nil || !ok {
		h.send(cb.Message.Chat.ID, "Admin panelga ruxsat yo'q.", nil)
		return
	}
	data := cb.Data
	switch {
	case data == "admin:home":
		h.edit(cb.Message.Chat.ID, cb.Message.MessageID, "Admin panel", telegram.AdminKeyboard())
	case data == "admin:settings":
		h.showSettings(ctx, cb.Message.Chat.ID, cb.Message.MessageID)
	case data == "admin:stats":
		h.showStats(ctx, cb.Message.Chat.ID, cb.Message.MessageID)
	case data == "admin:logs":
		h.showLogs(ctx, cb.Message.Chat.ID, cb.Message.MessageID)
	case data == "admin:admins":
		h.showAdmins(ctx, cb.Message.Chat.ID, cb.Message.MessageID)
	case data == "admin:add:ADMIN" || data == "admin:add:MODERATOR":
		if adminUser.Role != "SUPERADMIN" {
			h.send(cb.Message.Chat.ID, "Faqat SUPERADMIN admin qo'sha oladi.", nil)
			return
		}
		role := strings.TrimPrefix(data, "admin:add:")
		h.setPending(ctx, cb.From.ID, "add_admin:"+role)
		h.send(cb.Message.Chat.ID, "Telegram ID yuboring.", nil)
	case data == "admin:remove":
		if adminUser.Role != "SUPERADMIN" {
			h.send(cb.Message.Chat.ID, "Faqat SUPERADMIN admin o'chira oladi.", nil)
			return
		}
		h.setPending(ctx, cb.From.ID, "remove_admin")
		h.send(cb.Message.Chat.ID, "O'chiriladigan admin Telegram ID sini yuboring.", nil)
	case data == "admin:users":
		h.edit(cb.Message.Chat.ID, cb.Message.MessageID, "Userlar bo'limi: user status va umumiy son statistikada ko'rsatiladi.", telegram.AdminKeyboard())
	case data == "admin:downloads":
		h.edit(cb.Message.Chat.ID, cb.Message.MessageID, "Downloadlar bo'limi: queue va performance metrikalari statistikada jamlangan.", telegram.AdminKeyboard())
	case data == "admin:donate":
		h.setPending(ctx, cb.From.ID, "donate_text")
		h.send(cb.Message.Chat.ID, "Yangi donat matnini yuboring.", nil)
	case data == "admin:set:online":
		_ = h.settings.SetBool(ctx, "bot_online", true)
		h.adminLogs.Write(ctx, cb.From.ID, "settings.online", "online=true")
		h.showSettings(ctx, cb.Message.Chat.ID, cb.Message.MessageID)
	case data == "admin:set:offline":
		_ = h.settings.SetBool(ctx, "bot_online", false)
		h.adminLogs.Write(ctx, cb.From.ID, "settings.online", "online=false")
		h.showSettings(ctx, cb.Message.Chat.ID, cb.Message.MessageID)
	case data == "admin:set:maintenance":
		st, _ := h.settings.Get(ctx)
		_ = h.settings.SetBool(ctx, "maintenance_mode", !st.MaintenanceMode)
		h.adminLogs.Write(ctx, cb.From.ID, "settings.maintenance", fmt.Sprintf("maintenance=%v", !st.MaintenanceMode))
		h.showSettings(ctx, cb.Message.Chat.ID, cb.Message.MessageID)
	case data == "admin:telegram:cloud":
		_ = h.settings.SetText(ctx, "telegram_api_mode", "cloud")
		h.adminLogs.Write(ctx, cb.From.ID, "settings.telegram_api_mode", "cloud")
		h.showSettings(ctx, cb.Message.Chat.ID, cb.Message.MessageID)
	case data == "admin:telegram:local":
		_ = h.settings.SetText(ctx, "telegram_api_mode", "local")
		h.adminLogs.Write(ctx, cb.From.ID, "settings.telegram_api_mode", "local")
		h.showSettings(ctx, cb.Message.Chat.ID, cb.Message.MessageID)
	case data == "admin:telegram:health":
		h.showLocalBotAPIHealth(ctx, cb.Message.Chat.ID, cb.Message.MessageID)
	case data == "admin:max:video":
		h.edit(cb.Message.Chat.ID, cb.Message.MessageID, "Max video hajmni tanlang:", telegram.LimitPresetKeyboard("video", []int64{50, 100, 250, 500, 1024, 2048}))
	case data == "admin:max:audio":
		h.edit(cb.Message.Chat.ID, cb.Message.MessageID, "Max audio hajmni tanlang:", telegram.LimitPresetKeyboard("audio", []int64{20, 50, 100, 200}))
	case data == "admin:max:cloud_upload":
		h.edit(cb.Message.Chat.ID, cb.Message.MessageID, "Cloud upload limitni tanlang:", telegram.LimitPresetKeyboard("cloud_upload", []int64{20, 50}))
	case data == "admin:max:local_upload":
		h.edit(cb.Message.Chat.ID, cb.Message.MessageID, "Local upload limitni tanlang:", telegram.LimitPresetKeyboard("local_upload", []int64{50, 100, 500, 1024, 2000}))
	case strings.HasPrefix(data, "admin:limit:"):
		h.handleLimit(ctx, cb, adminUser)
	case strings.HasPrefix(data, "admin:edit:"):
		column := strings.TrimPrefix(data, "admin:edit:")
		h.setPending(ctx, cb.From.ID, "text:"+column)
		h.send(cb.Message.Chat.ID, "Yangi qiymatni yuboring.", nil)
	}
}

func (h *Handler) TryHandleInput(ctx context.Context, msg *tgbotapi.Message) bool {
	if msg.From == nil {
		return false
	}
	ok, _, err := h.admins.IsAdmin(ctx, msg.From.ID)
	if err != nil || !ok {
		return false
	}
	key := pendingKey(msg.From.ID)
	mode, err := h.redis.Get(ctx, key).Result()
	if err != nil {
		return false
	}
	defer h.redis.Del(ctx, key)
	switch {
	case strings.HasPrefix(mode, "limit:"):
		v, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil || v <= 0 {
			h.send(msg.Chat.ID, "To'g'ri MB sonini kiriting. Masalan: 300", nil)
			return true
		}
		column, ok := limitColumn(strings.TrimPrefix(mode, "limit:"))
		if !ok {
			return false
		}
		_ = h.settings.SetInt64(ctx, column, v)
		h.adminLogs.Write(ctx, msg.From.ID, "settings.limit", fmt.Sprintf("%s=%d", column, v))
		h.send(msg.Chat.ID, "Limit saqlandi.", nil)
	case strings.HasPrefix(mode, "text:"):
		column := strings.TrimPrefix(mode, "text:")
		_ = h.settings.SetText(ctx, column, msg.Text)
		h.adminLogs.Write(ctx, msg.From.ID, "settings.text", column)
		h.send(msg.Chat.ID, "Matn saqlandi.", nil)
	case mode == "donate_text":
		_ = h.settings.SetText(ctx, "donate_text", msg.Text)
		h.adminLogs.Write(ctx, msg.From.ID, "settings.donate_text", "updated")
		h.send(msg.Chat.ID, "Donat matni saqlandi.", nil)
	case strings.HasPrefix(mode, "add_admin:"):
		role := strings.TrimPrefix(mode, "add_admin:")
		telegramID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil || telegramID <= 0 {
			h.send(msg.Chat.ID, "To'g'ri Telegram ID kiriting.", nil)
			return true
		}
		_ = h.admins.Add(ctx, telegramID, role)
		h.adminLogs.Write(ctx, msg.From.ID, "admins.add", fmt.Sprintf("%d=%s", telegramID, role))
		h.send(msg.Chat.ID, "Admin saqlandi.", nil)
	case mode == "remove_admin":
		telegramID, err := strconv.ParseInt(strings.TrimSpace(msg.Text), 10, 64)
		if err != nil || telegramID <= 0 {
			h.send(msg.Chat.ID, "To'g'ri Telegram ID kiriting.", nil)
			return true
		}
		if telegramID == msg.From.ID {
			h.send(msg.Chat.ID, "O'zingizni o'chira olmaysiz.", nil)
			return true
		}
		_ = h.admins.Remove(ctx, telegramID)
		h.adminLogs.Write(ctx, msg.From.ID, "admins.remove", strconv.FormatInt(telegramID, 10))
		h.send(msg.Chat.ID, "Admin o'chirildi.", nil)
	default:
		return false
	}
	return true
}

func (h *Handler) handleLimit(ctx context.Context, cb *tgbotapi.CallbackQuery, adminUser users.Admin) {
	parts := strings.Split(cb.Data, ":")
	if len(parts) != 4 {
		return
	}
	kind := parts[2]
	value := parts[3]
	if value == "manual" {
		h.setPending(ctx, cb.From.ID, "limit:"+kind)
		h.send(cb.Message.Chat.ID, "MB miqdorini yuboring. Masalan: 300", nil)
		return
	}
	mb, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return
	}
	column, ok := limitColumn(kind)
	if !ok {
		return
	}
	_ = h.settings.SetInt64(ctx, column, mb)
	h.adminLogs.Write(ctx, adminUser.TelegramID, "settings.limit", fmt.Sprintf("%s=%d", column, mb))
	h.showSettings(ctx, cb.Message.Chat.ID, cb.Message.MessageID)
}

func (h *Handler) showLocalBotAPIHealth(ctx context.Context, chatID int64, messageID int) {
	if h.delivery == nil {
		h.edit(chatID, messageID, "Local Telegram Bot API health check mavjud emas.", telegram.AdminSettingsKeyboard(false, false, "cloud"))
		return
	}
	if err := h.delivery.HealthCheck(ctx); err != nil {
		h.edit(chatID, messageID, "Local Telegram Bot API server ishlamayapti.\n\n"+err.Error(), telegram.AdminKeyboard())
		return
	}
	h.edit(chatID, messageID, "Local Telegram Bot API server ishlayapti.", telegram.AdminKeyboard())
}

func limitColumn(kind string) (string, bool) {
	switch kind {
	case "video":
		return "max_video_file_size_mb", true
	case "audio":
		return "max_audio_file_size_mb", true
	case "cloud_upload":
		return "telegram_cloud_max_upload_mb", true
	case "local_upload":
		return "telegram_local_max_upload_mb", true
	default:
		return "", false
	}
}

func (h *Handler) setPending(ctx context.Context, telegramID int64, mode string) {
	_ = h.redis.Set(ctx, pendingKey(telegramID), mode, 10*time.Minute).Err()
}

func pendingKey(telegramID int64) string {
	return "admin_input:" + strconv.FormatInt(telegramID, 10)
}

func (h *Handler) send(chatID int64, text string, replyMarkup any) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = replyMarkup
	_, _ = h.bot.Send(msg)
}

func (h *Handler) edit(chatID int64, messageID int, text string, replyMarkup any) {
	msg := tgbotapi.NewEditMessageText(chatID, messageID, text)
	if markup, ok := replyMarkup.(tgbotapi.InlineKeyboardMarkup); ok {
		msg.ReplyMarkup = &markup
	}
	_, _ = h.bot.Send(msg)
}
