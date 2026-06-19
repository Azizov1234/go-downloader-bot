package workers

import (
	"context"
	"encoding/json"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hibiken/asynq"

	"instagram-downloader-bot/internal/media"
	"instagram-downloader-bot/internal/queue"
	"instagram-downloader-bot/internal/storage"
	"instagram-downloader-bot/internal/telegram"
	"instagram-downloader-bot/internal/users"
)

type SendWorker struct {
	bot      *tgbotapi.BotAPI
	delivery *media.DeliveryService
	media    *media.Service
	users    *users.Service
	queue    *queue.Client
	storage  storage.Service
	locks    *queue.Locks
}

func NewSendWorker(bot *tgbotapi.BotAPI, delivery *media.DeliveryService, mediaService *media.Service, usersService *users.Service, queueClient *queue.Client, storageService storage.Service, locks *queue.Locks) *SendWorker {
	return &SendWorker{bot: bot, delivery: delivery, media: mediaService, users: usersService, queue: queueClient, storage: storageService, locks: locks}
}

func (w *SendWorker) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload queue.SendTask
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}
	start := time.Now()
	mediaFile, err := w.media.GetOrCreateMediaFile(ctx, payload.OriginalURL, payload.NormalizedURL, payload.InstagramShortcode)
	if err != nil {
		return err
	}
	variant, err := w.media.FindVariant(ctx, payload.NormalizedURL, payload.VariantType, payload.Quality)
	if err != nil {
		variant, err = w.media.UpsertVariant(ctx, mediaFile, payload.VariantType, payload.Quality, payload.FileID, payload.UniqueID, payload.Metadata, "READY")
		if err != nil {
			return err
		}
	}

	if payload.FileID != "" {
		variant.TelegramFileID = payload.FileID
		variant.TelegramFileUniqueID = payload.UniqueID
		sent, err := w.delivery.SendByFileID(ctx, payload.Recipient.ChatID, variant, telegram.MediaActionsKeyboard(variant.ID))
		if err != nil {
			_ = w.media.ClearFileID(ctx, variant.ID)
			return w.queue.EnqueueDownload(ctx, payload.DownloadTask)
		}
		_ = sent
		w.markSuccess(ctx, payload.Recipient, variant.ID, payload, time.Since(start))
		return nil
	}

	waiters, _ := w.locks.PopWaiters(ctx, queue.WaitersKey(payload.NormalizedURL, payload.VariantType, payload.Quality))
	if len(waiters) == 0 {
		waiters = []queue.Recipient{payload.Recipient}
	}
	first := waiters[0]
	sent, err := w.delivery.SendLocal(ctx, first.ChatID, payload.LocalPath, variant, telegram.MediaActionsKeyboard(variant.ID))
	if err != nil {
		w.locks.Release(ctx, queue.LockKey(payload.NormalizedURL, payload.VariantType, payload.Quality))
		return err
	}
	variant, err = w.media.UpsertVariant(ctx, mediaFile, payload.VariantType, payload.Quality, sent.FileID, sent.FileUniqueID, payload.Metadata, "READY")
	if err != nil {
		w.locks.Release(ctx, queue.LockKey(payload.NormalizedURL, payload.VariantType, payload.Quality))
		return err
	}
	w.markSuccess(ctx, first, variant.ID, payload, sent.SendDuration)
	for _, r := range waiters[1:] {
		_, err := w.delivery.SendByFileID(ctx, r.ChatID, variant, telegram.MediaActionsKeyboard(variant.ID))
		if err == nil {
			w.markSuccess(ctx, r, variant.ID, payload, time.Since(start))
		}
		time.Sleep(80 * time.Millisecond)
	}
	_ = w.storage.RemoveSafe(payload.LocalPath)
	w.locks.Release(ctx, queue.LockKey(payload.NormalizedURL, payload.VariantType, payload.Quality))
	return nil
}

func (w *SendWorker) markSuccess(ctx context.Context, r queue.Recipient, variantID int64, payload queue.SendTask, sendDuration time.Duration) {
	if r.DownloadID > 0 {
		_ = w.media.UpdateDownloadMetrics(ctx, r.DownloadID, &variantID, "SUCCESS", time.Since(payload.QueuedAt), time.Duration(payload.DownloadMs)*time.Millisecond, time.Duration(payload.ConvertMs)*time.Millisecond, sendDuration, time.Since(payload.QueuedAt), "")
	}
	w.media.MarkDaily(ctx, payload.VariantType, payload.FileID != "", "SUCCESS", false)
	_ = w.users.IncrementDownloads(ctx, r.UserID)
}
