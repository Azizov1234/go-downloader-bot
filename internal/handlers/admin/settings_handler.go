package admin

import (
	"context"
	"fmt"

	"instagram-downloader-bot/internal/telegram"
)

func (h *Handler) showSettings(ctx context.Context, chatID int64, messageID int) {
	st, err := h.settings.Get(ctx)
	if err != nil {
		h.edit(chatID, messageID, "Sozlamalarni o'qib bo'lmadi.", telegram.AdminKeyboard())
		return
	}
	text := fmt.Sprintf("⚙️ Bot sozlamalari\n\nOnline: %v\nMaintenance: %v\nMax video: %d MB\nMax audio: %d MB\nTelegram upload: %d MB",
		st.BotOnline, st.MaintenanceMode, st.MaxVideoFileSizeMB, st.MaxAudioFileSizeMB, st.TelegramMaxUploadMB)
	h.edit(chatID, messageID, text, telegram.AdminSettingsKeyboard(st.BotOnline, st.MaintenanceMode))
}
