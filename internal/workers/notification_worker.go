package workers

import (
	"context"
	"encoding/json"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hibiken/asynq"

	"instagram-downloader-bot/internal/config"
	"instagram-downloader-bot/internal/queue"
)

type NotificationWorker struct {
	bot *tgbotapi.BotAPI
	cfg config.Config
}

func NewNotificationWorker(bot *tgbotapi.BotAPI, cfg config.Config) *NotificationWorker {
	return &NotificationWorker{bot: bot, cfg: cfg}
}

func (w *NotificationWorker) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload queue.NotificationTask
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}
	if w.cfg.SuperAdminTelegramID == 0 {
		return nil
	}
	msg := tgbotapi.NewMessage(w.cfg.SuperAdminTelegramID, payload.Text)
	_, err := w.bot.Send(msg)
	return err
}
