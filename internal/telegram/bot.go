package telegram

import (
	"context"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type UpdateHandler interface {
	HandleUpdate(ctx context.Context, update tgbotapi.Update)
}

type Bot struct {
	API    *tgbotapi.BotAPI
	Logger *slog.Logger
}

func New(token string, logger *slog.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Bot{API: api, Logger: logger}, nil
}

func (b *Bot) Start(ctx context.Context, handler UpdateHandler) {
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := b.API.GetUpdatesChan(updateConfig)
	b.Logger.Info("telegram bot started", "username", b.API.Self.UserName)
	for {
		select {
		case <-ctx.Done():
			b.API.StopReceivingUpdates()
			return
		case update := <-updates:
			go handler.HandleUpdate(ctx, update)
		}
	}
}
