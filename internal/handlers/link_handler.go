package handlers

import (
	"context"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"instagram-downloader-bot/internal/media"
	"instagram-downloader-bot/internal/telegram"
	"instagram-downloader-bot/internal/users"
	apperrors "instagram-downloader-bot/pkg/errors"
)

func (r *Router) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	if msg.From == nil || msg.Text == "" {
		return
	}
	if r.rateLimited(ctx, msg.From.ID) {
		r.send(msg.Chat.ID, "Juda ko'p so'rov yubordingiz. Bir ozdan keyin urinib ko'ring.", nil)
		return
	}
	if r.admin.TryHandleInput(ctx, msg) {
		return
	}
	if msg.IsCommand() {
		switch msg.Command() {
		case "start":
			r.handleStart(ctx, msg)
		case "admin":
			r.admin.Show(ctx, msg.Chat.ID, msg.From.ID)
		default:
			r.send(msg.Chat.ID, "Instagram havolasini yuboring.", telegram.UserMenu())
		}
		return
	}

	user, err := r.users.Upsert(ctx, msg.From.ID, msg.From.UserName, msg.From.FirstName+" "+msg.From.LastName)
	if err != nil {
		r.send(msg.Chat.ID, telegram.UniversalErrorMessage, nil)
		return
	}
	blocked, _ := r.users.IsBlocked(ctx, msg.From.ID)
	if blocked {
		r.send(msg.Chat.ID, "Akkount bloklangan.", nil)
		return
	}

	text := strings.TrimSpace(msg.Text)
	switch text {
	case "Havola yuborish":
		r.send(msg.Chat.ID, "Instagram link yuboring.", telegram.UserMenu())
		return
	case "Saqlanganlar":
		r.handleSaved(ctx, msg.Chat.ID, msg.From.ID, 1, 0)
		return
	case "Profil":
		r.handleProfile(ctx, msg)
		return
	case "Donat":
		r.handleDonate(ctx, msg)
		return
	case "Qo'llanma":
		st, _ := r.settings.Get(ctx)
		r.send(msg.Chat.ID, st.HelpText, telegram.UserMenu())
		return
	}

	if ok, _, _ := r.admins.IsAdmin(ctx, msg.From.ID); !ok {
		st, err := r.settings.Get(ctx)
		if err == nil && (!st.BotOnline || st.MaintenanceMode) {
			r.send(msg.Chat.ID, "Bot hozir texnik rejimda. Keyinroq urinib ko'ring.", nil)
			return
		}
	}

	if strings.Contains(text, "instagram.com") {
		r.handleInstagramLink(ctx, msg, user, extractInstagramURL(text))
		return
	}
	r.send(msg.Chat.ID, telegram.UnsupportedPlatformMessage, nil)
}

func extractInstagramURL(text string) string {
	for _, field := range strings.Fields(text) {
		cleaned := strings.Trim(field, " \n\t.,;()[]{}<>\"'")
		if strings.Contains(cleaned, "instagram.com") {
			return cleaned
		}
	}
	return text
}

func (r *Router) handleInstagramLink(ctx context.Context, msg *tgbotapi.Message, user users.User, raw string) {
	requestStarted := time.Now()
	parsed, err := r.provider.Parse(raw)
	if err != nil {
		if err == apperrors.ErrUnsupportedPlatform {
			r.send(msg.Chat.ID, telegram.UnsupportedPlatformMessage, nil)
			return
		}
		r.send(msg.Chat.ID, telegram.UnsupportedPlatformMessage, nil)
		return
	}

	// Link darhol saqlanadi. Media file_id esa birinchi muvaffaqiyatli uploaddan keyin cache bo'ladi.
	_, _ = r.media.GetOrCreateMediaFile(ctx, parsed.OriginalURL, parsed.NormalizedURL, parsed.Shortcode)

	cacheResult := r.cache.Lookup(ctx, parsed.NormalizedURL, media.VariantVideo, media.QualityAuto)
	if cacheResult.Hit {
		variantID := cacheResult.Variant.ID
		downloadID, _ := r.media.CreateDownload(ctx, user.ID, &variantID, "SUCCESS", true, cacheResult.Took)
		_ = downloadID
		_, sendErr := r.delivery.SendByFileIDTimed(ctx, msg.Chat.ID, cacheResult.Variant, telegram.MediaActionsKeyboard(cacheResult.Variant.ID), time.Since(requestStarted))
		if sendErr == nil {
			r.media.MarkDaily(ctx, media.VariantVideo, true, "SUCCESS", false)
			_ = r.users.IncrementDownloads(ctx, user.ID)
			return
		}
		_ = r.media.ClearFileID(ctx, cacheResult.Variant.ID)
		r.logs.Write(ctx, &user.ID, "instagram", "cache_send", "cached file_id ishlamadi", sendErr)
	}

	st := selectionState{
		OriginalURL: parsed.OriginalURL, NormalizedURL: parsed.NormalizedURL, InstagramShortcode: parsed.Shortcode,
		VariantType: media.VariantVideo, Quality: media.QualityAuto, UserID: user.ID, ChatID: msg.Chat.ID,
	}
	token, err := r.setSelection(ctx, st)
	if err != nil {
		r.send(msg.Chat.ID, telegram.UniversalErrorMessage, nil)
		return
	}
	r.send(msg.Chat.ID, telegram.SelectionText(st.VariantType, st.Quality), telegram.SelectionKeyboard(token, st.VariantType, st.Quality))
}
