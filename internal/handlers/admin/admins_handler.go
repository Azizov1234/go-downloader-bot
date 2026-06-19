package admin

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"instagram-downloader-bot/internal/telegram"
)

func (h *Handler) showAdmins(ctx context.Context, chatID int64, messageID int) {
	admins, err := h.admins.List(ctx)
	if err != nil {
		h.edit(chatID, messageID, "Adminlarni o'qib bo'lmadi.", telegram.AdminKeyboard())
		return
	}
	var lines []string
	for _, a := range admins {
		lines = append(lines, fmt.Sprintf("%d - %s", a.TelegramID, a.Role))
	}
	if len(lines) == 0 {
		lines = append(lines, "Admin yo'q")
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("➕ ADMIN", "admin:add:ADMIN"),
			tgbotapi.NewInlineKeyboardButtonData("➕ MODERATOR", "admin:add:MODERATOR"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🗑 O'chirish", "admin:remove"),
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Orqaga", "admin:home"),
		),
	)
	h.edit(chatID, messageID, "👮 Adminlar\n\n"+strings.Join(lines, "\n"), kb)
}
