package handlers

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"instagram-downloader-bot/internal/telegram"
)

func (r *Router) handleProfile(ctx context.Context, msg *tgbotapi.Message) {
	p, err := r.users.Profile(ctx, msg.From.ID)
	if err != nil {
		r.send(msg.Chat.ID, telegram.UniversalErrorMessage, nil)
		return
	}
	r.send(msg.Chat.ID, telegram.ProfileText(p), telegram.UserMenu())
}

func (r *Router) handleDonate(ctx context.Context, msg *tgbotapi.Message) {
	text, err := r.donate.Text(ctx)
	if err != nil {
		r.send(msg.Chat.ID, telegram.UniversalErrorMessage, nil)
		return
	}
	r.send(msg.Chat.ID, text, telegram.UserMenu())
}

func (r *Router) handleSaved(ctx context.Context, chatID, telegramID int64, page int, messageID int) {
	user, err := r.users.GetByTelegramID(ctx, telegramID)
	if err != nil {
		r.send(chatID, telegram.UniversalErrorMessage, nil)
		return
	}
	items, total, err := r.saved.List(ctx, user.ID, page, 10)
	if err != nil {
		r.send(chatID, telegram.UniversalErrorMessage, nil)
		return
	}
	text := telegram.SavedListText(items, page, total)
	kb := telegram.SavedPaginationKeyboard(page, total, 10)
	if messageID > 0 {
		r.edit(chatID, messageID, text, kb)
		return
	}
	r.send(chatID, text, kb)
}
