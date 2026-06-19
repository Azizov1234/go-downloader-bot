package handlers

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"instagram-downloader-bot/internal/telegram"
)

func (r *Router) handleStart(ctx context.Context, msg *tgbotapi.Message) {
	user, err := r.users.Upsert(ctx, msg.From.ID, msg.From.UserName, msg.From.FirstName+" "+msg.From.LastName)
	if err != nil {
		r.logger.Error("upsert user failed", "error", err)
		r.send(msg.Chat.ID, telegram.UniversalErrorMessage, nil)
		return
	}
	_ = user
	st, err := r.settings.Get(ctx)
	if err != nil {
		r.send(msg.Chat.ID, "Instagram havolasini yuboring.", telegram.UserMenu())
		return
	}
	r.send(msg.Chat.ID, st.WelcomeText, telegram.UserMenu())
}
