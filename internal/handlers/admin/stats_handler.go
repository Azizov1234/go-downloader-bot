package admin

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (h *Handler) showStats(ctx context.Context, chatID int64, messageID int) {
	text, err := h.stats.Summary(ctx)
	if err != nil {
		text = "Statistikani o'qib bo'lmadi."
	}
	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("⬅️ Orqaga", "admin:home")),
	)
	h.edit(chatID, messageID, text, kb)
}
