package admin

import (
	"context"
	"strings"

	"instagram-downloader-bot/internal/telegram"
)

func (h *Handler) showLogs(ctx context.Context, chatID int64, messageID int) {
	lines, err := h.logs.Recent(ctx, 10)
	if err != nil || len(lines) == 0 {
		h.edit(chatID, messageID, "Loglar topilmadi.", telegram.AdminKeyboard())
		return
	}
	h.edit(chatID, messageID, "🧾 Loglar\n\n"+strings.Join(lines, "\n\n"), telegram.AdminKeyboard())
}
