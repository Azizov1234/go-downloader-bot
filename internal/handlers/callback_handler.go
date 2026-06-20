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
	case strings.HasPrefix(data, "sel:"):
		r.handleSelectionCallback(ctx, cb)
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

func (r *Router) handleSelectionCallback(ctx context.Context, cb *tgbotapi.CallbackQuery) {
	parts := strings.Split(cb.Data, ":")
	if len(parts) < 3 {
		return
	}
	token := parts[1]
	action := parts[2]
	st, err := r.getSelection(ctx, token)
	if err != nil {
		r.edit(cb.Message.Chat.ID, cb.Message.MessageID, "Tanlov eskirgan. Linkni qayta yuboring.", nil)
		return
	}

	switch action {
	case "type":
		if len(parts) < 4 {
			return
		}
		if parts[3] == "audio" {
			st.VariantType = media.VariantAudio
			st.Quality = media.QualityMP3
		} else {
			st.VariantType = media.VariantVideo
			if st.Quality == media.QualityMP3 {
				st.Quality = media.QualityAuto
			}
		}
		_ = r.saveSelection(ctx, token, st)
		r.edit(cb.Message.Chat.ID, cb.Message.MessageID, telegram.SelectionText(st.VariantType, st.Quality), telegram.SelectionKeyboard(token, st.VariantType, st.Quality))
	case "q":
		if len(parts) < 4 {
			return
		}
		st.VariantType = media.VariantVideo
		st.Quality = media.Quality(parts[3])
		_ = r.saveSelection(ctx, token, st)
		r.edit(cb.Message.Chat.ID, cb.Message.MessageID, telegram.SelectionText(st.VariantType, st.Quality), telegram.SelectionKeyboard(token, st.VariantType, st.Quality))
	case "cancel":
		r.deleteSelection(ctx, token)
		r.edit(cb.Message.Chat.ID, cb.Message.MessageID, "Bekor qilindi.", nil)
	case "download":
		r.enqueueSelected(ctx, cb, token, st)
	}
}

func (r *Router) enqueueSelected(ctx context.Context, cb *tgbotapi.CallbackQuery, token string, st selectionState) {
	cacheResult := r.cache.Lookup(ctx, st.NormalizedURL, st.VariantType, st.Quality)
	if cacheResult.Hit {
		variantID := cacheResult.Variant.ID
		_, _ = r.media.CreateDownload(ctx, st.UserID, &variantID, "SUCCESS", true, cacheResult.Took)
		_, err := r.delivery.SendByFileID(ctx, st.ChatID, cacheResult.Variant, telegram.MediaActionsKeyboard(cacheResult.Variant.ID))
		if err == nil {
			r.media.MarkDaily(ctx, st.VariantType, true, "SUCCESS", false)
			_ = r.users.IncrementDownloads(ctx, st.UserID)
			r.deleteSelection(ctx, token)
			r.edit(cb.Message.Chat.ID, cb.Message.MessageID, "Cache orqali yuborildi ✅", nil)
			return
		}
		_ = r.media.ClearFileID(ctx, cacheResult.Variant.ID)
		r.logs.Write(ctx, &st.UserID, "instagram", "cache_send", "cached file_id ishlamadi", err)
	}

	limitReached, err := r.users.DailyLimitReached(ctx, st.UserID, r.cfg.DailyUserDownloadLimit)
	if err != nil {
		r.send(st.ChatID, telegram.UniversalErrorMessage, nil)
		return
	}
	if limitReached {
		r.send(st.ChatID, "Kunlik yuklab olish limiti tugadi.", nil)
		return
	}
	downloadID, err := r.media.CreateDownload(ctx, st.UserID, nil, "QUEUED", false, cacheResult.Took)
	if err != nil {
		r.send(st.ChatID, telegram.UniversalErrorMessage, nil)
		return
	}
	task := queue.DownloadTask{
		Recipient:  queue.Recipient{ChatID: st.ChatID, UserID: st.UserID, DownloadID: downloadID, Username: cb.From.UserName},
		DownloadID: downloadID, OriginalURL: st.OriginalURL, NormalizedURL: st.NormalizedURL,
		InstagramShortcode: st.InstagramShortcode, VariantType: st.VariantType, Quality: st.Quality, QueuedAt: time.Now(),
	}
	if err := r.queue.EnqueueDownload(ctx, task); err != nil {
		r.logs.Write(ctx, &st.UserID, "instagram", "enqueue", "queuega qo'shib bo'lmadi", err)
		r.send(st.ChatID, telegram.UniversalErrorMessage, nil)
		return
	}
	r.deleteSelection(ctx, token)
	r.edit(cb.Message.Chat.ID, cb.Message.MessageID, "⏳ Yuklash navbatga qo'shildi. Tayyor bo'lgach yuboraman.", nil)
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
	case "quality":
		st := selectionState{
			OriginalURL: variant.OriginalURL, NormalizedURL: variant.NormalizedURL, InstagramShortcode: variant.InstagramShortcode,
			VariantType: media.VariantVideo, Quality: media.QualityAuto, UserID: user.ID, ChatID: cb.Message.Chat.ID,
		}
		token, err := r.setSelection(ctx, st)
		if err != nil {
			r.send(cb.Message.Chat.ID, telegram.UniversalErrorMessage, nil)
			return
		}
		r.send(cb.Message.Chat.ID, telegram.SelectionText(st.VariantType, st.Quality), telegram.SelectionKeyboard(token, st.VariantType, st.Quality))
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
		Recipient:  queue.Recipient{ChatID: cb.Message.Chat.ID, UserID: userID, DownloadID: downloadID, Username: cb.From.UserName},
		DownloadID: downloadID, OriginalURL: variant.OriginalURL, NormalizedURL: variant.NormalizedURL,
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
