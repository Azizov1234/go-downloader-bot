package admin

import (
	"context"

	"instagram-downloader-bot/internal/telegram"
)

func (h *Handler) showStats(ctx context.Context, chatID int64, messageID int) {
	text, err := h.stats.Summary(ctx)
	if err != nil {
		text = "Statistikani o'qib bo'lmadi."
	}
	h.edit(chatID, messageID, text, telegram.AdminKeyboard())
}
