package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"instagram-downloader-bot/internal/media"
	"instagram-downloader-bot/internal/queue"
	"instagram-downloader-bot/internal/telegram"
)

func (r *Router) handleCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	if cb.From == nil || cb.Message == nil {
		return
	}
	data := cb.Data
	r.answerCallback(cb.ID, "")
	switch {
	case strings.HasPrefix(data, "media:"):
		r.handleMediaCallback(ctx, cb)
	case strings.HasPrefix(data, "saved:"):
		page, _ := strconv.Atoi(strings.TrimPrefix(data, "saved:"))
		if page < 1 {
			page = 1
		}
		r.handleSaved(ctx, cb.Message.Chat.ID, cb.From.ID, page, cb.Message.MessageID)
	case strings.HasPrefix(data, "admin:"):
		r.admin.HandleCallback(ctx, cb)
	}
}

func (r *Router) handleMediaCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	parts := strings.Split(cb.Data, ":")
	if len(parts) != 3 {
		return
	}
	variantID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return
	}
	action := parts[2]
	variant, err := r.media.GetVariantByID(ctx, variantID)
	if err != nil {
		r.send(cb.Message.Chat.ID, telegram.UniversalErrorMessage, nil)
		return
	}
	user, err := r.users.GetByTelegramID(ctx, cb.From.ID)
	if err != nil {
		r.send(cb.Message.Chat.ID, telegram.UniversalErrorMessage, nil)
		return
	}
	switch action {
	case "save":
		item, err := r.saved.Save(ctx, user.ID, variant)
		if err != nil {
			r.send(cb.Message.Chat.ID, telegram.UniversalErrorMessage, nil)
			return
		}
		r.send(cb.Message.Chat.ID, "Saqlab qo'yildi: "+fmt.Sprintf("#%06d", item.SaveNumber), nil)
	case "info":
		text := fmt.Sprintf("📄 Ma'lumot\n\nPlatform: instagram\nType: %s\nQuality: %s\nSize: %s\nDuration: %s\nURL: %s",
			variant.VariantType, variant.Quality, formatSize(variant.FileSize), formatDuration(variant.Duration), variant.NormalizedURL)
		r.send(cb.Message.Chat.ID, text, nil)
	case "mp3":
		r.enqueueMP3(ctx, cb, user.ID, variant)
	case "share":
		r.send(cb.Message.Chat.ID, "Ulashish uchun link:\n"+variant.NormalizedURL, nil)
	case "delete":
		del := tgbotapi.NewDeleteMessage(cb.Message.Chat.ID, cb.Message.MessageID)
		_, _ = r.bot.Request(del)
	}
}

func (r *Router) enqueueMP3(ctx context.Context, cb *tgbotapi.CallbackQuery, userID int64, variant media.MediaVariant) {
	downloadID, err := r.media.CreateDownload(ctx, userID, nil, "QUEUED", false, 0)
	if err != nil {
		r.send(cb.Message.Chat.ID, telegram.UniversalErrorMessage, nil)
		return
	}
	task := queue.DownloadTask{
		Recipient:          queue.Recipient{ChatID: cb.Message.Chat.ID, UserID: userID, DownloadID: downloadID, Username: cb.From.UserName},
		DownloadID:         downloadID, OriginalURL: variant.OriginalURL, NormalizedURL: variant.NormalizedURL,
		InstagramShortcode: variant.InstagramShortcode, VariantType: media.VariantAudio, Quality: media.QualityMP3, QueuedAt: time.Now(),
	}
	if err := r.queue.EnqueueDownload(ctx, task); err != nil {
		r.send(cb.Message.Chat.ID, telegram.UniversalErrorMessage, nil)
		return
	}
	r.send(cb.Message.Chat.ID, "🎵 MP3 navbatga qo'shildi.", nil)
}

func formatSize(v *int64) string {
	if v == nil || *v == 0 {
		return "noma'lum"
	}
	return fmt.Sprintf("%.1f MB", float64(*v)/1024/1024)
}

func formatDuration(v *int) string {
	if v == nil || *v == 0 {
		return "noma'lum"
	}
	return fmt.Sprintf("%d s", *v)
}
