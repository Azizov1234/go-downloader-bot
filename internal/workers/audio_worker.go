package workers

import (
	"context"
	"encoding/json"
	"os"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hibiken/asynq"

	"instagram-downloader-bot/internal/downloader"
	"instagram-downloader-bot/internal/media"
	"instagram-downloader-bot/internal/queue"
	"instagram-downloader-bot/internal/settings"
	"instagram-downloader-bot/internal/storage"
	"instagram-downloader-bot/internal/telegram"
)

type AudioWorker struct {
	bot      *tgbotapi.BotAPI
	ffmpeg   downloader.FFMpeg
	storage  storage.Service
	settings *settings.Service
	media    *media.Service
	queue    *queue.Client
	locks    *queue.Locks
}

func NewAudioWorker(bot *tgbotapi.BotAPI, ffmpeg downloader.FFMpeg, storageService storage.Service, settingsService *settings.Service, mediaService *media.Service, queueClient *queue.Client, locks *queue.Locks) *AudioWorker {
	return &AudioWorker{bot: bot, ffmpeg: ffmpeg, storage: storageService, settings: settingsService, media: mediaService, queue: queueClient, locks: locks}
}

func (w *AudioWorker) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload queue.AudioConvertTask
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}
	start := time.Now()
	if err := w.ffmpeg.ToMP3(ctx, payload.SourcePath, payload.OutputPath); err != nil {
		_ = w.storage.RemoveSafe(payload.SourcePath)
		w.locks.Release(ctx, queue.LockKey(payload.NormalizedURL, payload.VariantType, payload.Quality))
		return err
	}
	_ = w.storage.RemoveSafe(payload.SourcePath)
	stat, _ := os.Stat(payload.OutputPath)
	size := int64(0)
	if stat != nil {
		size = stat.Size()
	}
	st, err := w.settings.Get(ctx)
	if err != nil {
		return err
	}
	audioLimit := minPositive(st.MaxAudioFileSizeMB, st.TelegramMaxUploadMB)
	if size > 0 && bytesToMB(size) > audioLimit {
		_ = w.storage.RemoveSafe(payload.OutputPath)
		text := telegram.TooLargeAudio(audioLimit, bytesToMB(size))
		waiters, _ := w.locks.PopWaiters(ctx, queue.WaitersKey(payload.NormalizedURL, payload.VariantType, payload.Quality))
		if len(waiters) == 0 {
			waiters = []queue.Recipient{payload.Recipient}
		}
		for _, r := range waiters {
			msg := tgbotapi.NewMessage(r.ChatID, text)
			_, _ = w.bot.Send(msg)
			if r.DownloadID > 0 {
				_ = w.media.UpdateDownloadMetrics(ctx, r.DownloadID, nil, "FAILED", time.Since(payload.QueuedAt), time.Duration(payload.DownloadMs)*time.Millisecond, time.Since(start), 0, time.Since(payload.QueuedAt), "oversized")
			}
		}
		w.media.MarkDaily(ctx, media.VariantAudio, false, "FAILED", true)
		w.locks.Release(ctx, queue.LockKey(payload.NormalizedURL, payload.VariantType, payload.Quality))
		return nil
	}
	md := media.Metadata{FileSize: size}
	return w.queue.EnqueueSend(ctx, queue.SendTask{DownloadTask: payload.DownloadTask, LocalPath: payload.OutputPath, Metadata: md, DownloadMs: payload.DownloadMs, ConvertMs: time.Since(start).Milliseconds()})
}
