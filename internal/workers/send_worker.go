package workers

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hibiken/asynq"

	"instagram-downloader-bot/internal/logs"
	"instagram-downloader-bot/internal/media"
	"instagram-downloader-bot/internal/queue"
	"instagram-downloader-bot/internal/storage"
	"instagram-downloader-bot/internal/telegram"
	"instagram-downloader-bot/internal/users"
)

type SendWorker struct {
	bot      *tgbotapi.BotAPI
	logger   *slog.Logger
	delivery *media.DeliveryService
	media    *media.Service
	users    *users.Service
	queue    *queue.Client
	storage  storage.Service
	locks    *queue.Locks
	logs     *logs.ErrorLogService
}

func NewSendWorker(bot *tgbotapi.BotAPI, logger *slog.Logger, delivery *media.DeliveryService, mediaService *media.Service, usersService *users.Service, queueClient *queue.Client, storageService storage.Service, locks *queue.Locks, logsService *logs.ErrorLogService) *SendWorker {
	return &SendWorker{bot: bot, logger: logger, delivery: delivery, media: mediaService, users: usersService, queue: queueClient, storage: storageService, locks: locks, logs: logsService}
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
		sent, err := w.delivery.SendByFileIDTimed(ctx, payload.Recipient.ChatID, variant, telegram.MediaActionsKeyboard(variant.ID), time.Since(payload.QueuedAt), payload.CustomTitle)
		if err != nil {
			_ = w.media.ClearFileID(ctx, variant.ID)
			return w.queue.EnqueueDownload(ctx, payload.DownloadTask)
		}
		_ = sent
		w.markSuccess(ctx, payload.Recipient, variant.ID, payload, time.Since(start))
		w.logger.Info("delivery timing summary (cached)",
			"url", payload.OriginalURL,
			"probe_ms", int64(0),
			"download_ms", int64(0),
			"ffmpeg_ms", int64(0),
			"convert_ms", int64(0),
			"send_ms", time.Since(start).Milliseconds(),
			"total_ms", time.Since(payload.QueuedAt).Milliseconds(),
		)
		return nil
	}

	// Rename local file if custom title is provided
	if payload.CustomTitle != "" {
		safeTitle := sanitizeFilename(payload.CustomTitle)
		if safeTitle != "" {
			ext := filepath.Ext(payload.LocalPath)
			newPath := filepath.Join(filepath.Dir(payload.LocalPath), safeTitle+ext)
			if err := os.Rename(payload.LocalPath, newPath); err == nil {
				payload.LocalPath = newPath
			}
		}
	}

	waiters, _ := w.locks.PopWaiters(ctx, queue.WaitersKey(payload.NormalizedURL, payload.VariantType, payload.Quality))
	if len(waiters) == 0 {
		waiters = []queue.Recipient{payload.Recipient}
	}
	first := waiters[0]
	sent, err := w.delivery.SendLocalTimed(ctx, first.ChatID, payload.LocalPath, variant, telegram.MediaActionsKeyboard(variant.ID), time.Since(payload.QueuedAt), payload.CustomTitle)
	if err != nil {
		w.handleLocalSendFailure(ctx, payload, waiters, variant.ID, err)
		w.locks.Release(ctx, queue.LockKey(payload.NormalizedURL, payload.VariantType, payload.Quality))
		if !w.delivery.Config().KeepFailedDownloads {
			_ = w.storage.RemoveSafe(payload.LocalPath)
		}
		return nil
	}
	variant, err = w.media.UpsertVariant(ctx, mediaFile, payload.VariantType, payload.Quality, sent.FileID, sent.FileUniqueID, payload.Metadata, "READY")
	if err != nil {
		w.locks.Release(ctx, queue.LockKey(payload.NormalizedURL, payload.VariantType, payload.Quality))
		return err
	}
	w.markSuccess(ctx, first, variant.ID, payload, sent.SendDuration)

	w.logger.Info("delivery timing summary (fresh)",
		"url", payload.OriginalURL,
		"probe_ms", payload.ProbeMs,
		"download_ms", payload.DownloadMs,
		"ffmpeg_ms", payload.FFmpegMs,
		"convert_ms", payload.ConvertMs,
		"send_ms", sent.SendDuration.Milliseconds(),
		"total_ms", time.Since(payload.QueuedAt).Milliseconds(),
	)

	for _, r := range waiters[1:] {
		_, err := w.delivery.SendByFileIDTimed(ctx, r.ChatID, variant, telegram.MediaActionsKeyboard(variant.ID), time.Since(payload.QueuedAt), payload.CustomTitle)
		if err == nil {
			w.markSuccess(ctx, r, variant.ID, payload, time.Since(start))
		}
		time.Sleep(80 * time.Millisecond)
	}
	_ = w.storage.RemoveSafe(payload.LocalPath)
	w.locks.Release(ctx, queue.LockKey(payload.NormalizedURL, payload.VariantType, payload.Quality))
	return nil
}

func (w *SendWorker) handleLocalSendFailure(ctx context.Context, payload queue.SendTask, waiters []queue.Recipient, variantID int64, sendErr error) {
	text := telegram.SendFailedMessage
	oversized := isRequestTooLarge(sendErr) || media.IsTelegramFileTooLarge(sendErr)

	st, _ := w.delivery.CurrentSettings(ctx)
	mode := "cloud"
	if st.TelegramAPIMode != "" {
		mode = strings.ToLower(strings.TrimSpace(st.TelegramAPIMode))
	}
	limitMB := int64(50)
	if mode == "local" {
		if st.TelegramLocalMaxUploadMB > 0 {
			limitMB = st.TelegramLocalMaxUploadMB
		} else {
			limitMB = 2000
		}
	} else {
		if st.TelegramCloudMaxUploadMB > 0 {
			limitMB = st.TelegramCloudMaxUploadMB
		}
	}

	if limitMBVal, sizeMBVal, ok := media.TelegramLimit(sendErr); ok {
		if mode == "local" {
			text = telegram.TooLargeVideo(limitMBVal, sizeMBVal)
		} else {
			text = telegram.CloudVideoTooLarge(limitMBVal, sizeMBVal)
		}
	} else if isRequestTooLarge(sendErr) {
		text = telegram.TelegramUploadTooLarge(mode, limitMB, bytesToMB(payload.Metadata.FileSize))
	}

	if media.IsLocalBotAPIUnavailable(sendErr) {
		apiURL := w.delivery.Config().TelegramLocalAPIURL
		text = telegram.LocalBotAPIUnavailable(apiURL)
		_ = w.queue.EnqueueNotification(ctx, queue.NotificationTask{Text: "Local Bot API server ishlamayapti: " + sendErr.Error()})
	}
	for _, r := range waiters {
		msg := tgbotapi.NewMessage(r.ChatID, text)
		_, _ = w.bot.Send(msg)
		if r.DownloadID > 0 {
			_ = w.media.UpdateDownloadMetrics(ctx, r.DownloadID, &variantID, "FAILED", time.Since(payload.QueuedAt), time.Duration(payload.DownloadMs)*time.Millisecond, time.Duration(payload.ConvertMs)*time.Millisecond, 0, time.Since(payload.QueuedAt), sendErr.Error())
		}
		if w.logs != nil {
			userID := r.UserID
			w.logs.Write(ctx, &userID, "instagram", "telegram_send", text, sendErr)
		}
	}
	w.media.MarkDaily(ctx, payload.VariantType, false, "FAILED", oversized)
}

func isRequestTooLarge(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "request entity too large") || strings.Contains(msg, "too large")
}

func (w *SendWorker) markSuccess(ctx context.Context, r queue.Recipient, variantID int64, payload queue.SendTask, sendDuration time.Duration) {
	if r.DownloadID > 0 {
		_ = w.media.UpdateDownloadMetrics(ctx, r.DownloadID, &variantID, "SUCCESS", time.Since(payload.QueuedAt), time.Duration(payload.DownloadMs)*time.Millisecond, time.Duration(payload.ConvertMs)*time.Millisecond, sendDuration, time.Since(payload.QueuedAt), "")
	}
	w.media.MarkDaily(ctx, payload.VariantType, payload.FileID != "", "SUCCESS", false)
	_ = w.users.IncrementDownloads(ctx, r.UserID)
}

func sanitizeFilename(s string) string {
	r := strings.NewReplacer(
		"/", "",
		"\\", "",
		":", "",
		"*", "",
		"?", "",
		"\"", "",
		"<", "",
		">", "",
		"|", "",
	)
	return strings.TrimSpace(r.Replace(s))
}
