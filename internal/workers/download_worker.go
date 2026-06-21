package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hibiken/asynq"

	"instagram-downloader-bot/internal/downloader"
	"instagram-downloader-bot/internal/instagram"
	"instagram-downloader-bot/internal/logs"
	"instagram-downloader-bot/internal/media"
	"instagram-downloader-bot/internal/queue"
	"instagram-downloader-bot/internal/settings"
	"instagram-downloader-bot/internal/storage"
	"instagram-downloader-bot/internal/telegram"
	"instagram-downloader-bot/internal/users"
)

type DownloadWorker struct {
	bot            *tgbotapi.BotAPI
	logger         *slog.Logger
	downloader     downloader.Downloader
	ffprobe        downloader.FFProbe
	formats        instagram.FormatBuilder
	cookies        instagram.Cookies
	storage        storage.Service
	media          *media.Service
	settings       *settings.Service
	users          *users.Service
	logs           *logs.ErrorLogService
	queue          *queue.Client
	locks          *queue.Locks
	allowOversized bool
}

type DownloadWorkerDeps struct {
	Bot            *tgbotapi.BotAPI
	Logger         *slog.Logger
	Downloader     downloader.Downloader
	FFProbe        downloader.FFProbe
	Formats        instagram.FormatBuilder
	Cookies        instagram.Cookies
	Storage        storage.Service
	Media          *media.Service
	Settings       *settings.Service
	Users          *users.Service
	Logs           *logs.ErrorLogService
	Queue          *queue.Client
	Locks          *queue.Locks
	AllowOversized bool
}

func NewDownloadWorker(dep DownloadWorkerDeps) *DownloadWorker {
	return &DownloadWorker{
		bot: dep.Bot, logger: dep.Logger, downloader: dep.Downloader, ffprobe: dep.FFProbe, formats: dep.Formats,
		cookies: dep.Cookies, storage: dep.Storage, media: dep.Media, settings: dep.Settings,
		users: dep.Users, logs: dep.Logs, queue: dep.Queue, locks: dep.Locks,
		allowOversized: dep.AllowOversized,
	}
}

func (w *DownloadWorker) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload queue.DownloadTask
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	if cached, err := w.media.FindCachedVariant(ctx, payload.NormalizedURL, payload.VariantType, payload.Quality); err == nil {
		return w.queue.EnqueueSend(ctx, queue.SendTask{DownloadTask: payload, FileID: cached.TelegramFileID, UniqueID: cached.TelegramFileUniqueID})
	}

	lockKey := queue.LockKey(payload.NormalizedURL, payload.VariantType, payload.Quality)
	waitersKey := queue.WaitersKey(payload.NormalizedURL, payload.VariantType, payload.Quality)
	acquired, err := w.locks.Acquire(ctx, lockKey)
	if err != nil {
		return err
	}
	if !acquired {
		return w.locks.AddWaiter(ctx, waitersKey, payload.Recipient)
	}
	if err := w.locks.AddWaiter(ctx, waitersKey, payload.Recipient); err != nil {
		w.locks.Release(ctx, lockKey)
		return err
	}

	start := time.Now()
	st, err := w.settings.Get(ctx)
	if err != nil {
		w.failWaiters(ctx, payload, telegram.UniversalErrorMessage, err)
		w.locks.Release(ctx, lockKey)
		return err
	}
	format := w.formats.For(payload.VariantType, payload.Quality)
	videoLimit := effectiveUploadLimit(st.MaxVideoFileSizeMB, telegramUploadLimit(st), w.allowOversized)
	if payload.VariantType == media.VariantVideo && videoLimit > 0 {
		info, probeErr := w.downloader.Probe(ctx, payload.OriginalURL, format, w.cookies.Args())
		if probeErr != nil {
			w.logger.Warn("yt-dlp probe skipped after error", "error", probeErr)
		}
		size := knownSize(info)
		if size > 0 && bytesToMB(size) > videoLimit {
			sizeMB := bytesToMB(size)
			w.failWaiters(ctx, payload, tooLargeVideoText(st, videoLimit, sizeMB), nil)
			_ = w.media.UpdateDownloadMetrics(ctx, payload.DownloadID, nil, "FAILED", time.Since(payload.QueuedAt), 0, 0, 0, time.Since(start), "oversized")
			w.media.MarkDaily(ctx, payload.VariantType, false, "FAILED", true)
			w.locks.Release(ctx, lockKey)
			return nil
		}
	}

	dir, base, err := w.storage.DownloadBase(payload.Recipient.ChatID, payload.DownloadID, payload.Quality)
	if err != nil {
		w.failWaiters(ctx, payload, telegram.UniversalErrorMessage, err)
		w.locks.Release(ctx, lockKey)
		return err
	}
	localPath, err := w.downloader.Download(ctx, payload.OriginalURL, format, dir, base, w.cookies.Args())
	if err != nil {
		w.failWaiters(ctx, payload, telegram.InstagramErrorMessage, err)
		_ = w.media.UpdateDownloadMetrics(ctx, payload.DownloadID, nil, "FAILED", time.Since(payload.QueuedAt), time.Since(start), 0, 0, time.Since(start), err.Error())
		w.media.MarkDaily(ctx, payload.VariantType, false, "FAILED", false)
		w.locks.Release(ctx, lockKey)
		return nil
	}

	if payload.VariantType == media.VariantAudio {
		outPath, err := w.storage.AudioPath(payload.Recipient.ChatID, payload.DownloadID)
		if err != nil {
			_ = w.storage.RemoveSafe(localPath)
			w.failWaiters(ctx, payload, telegram.UniversalErrorMessage, err)
			w.locks.Release(ctx, lockKey)
			return err
		}
		return w.queue.EnqueueAudioConvert(ctx, queue.AudioConvertTask{DownloadTask: payload, SourcePath: localPath, OutputPath: outPath, DownloadMs: time.Since(start).Milliseconds()})
	}

	md, err := w.ffprobe.Metadata(ctx, localPath)
	if err != nil {
		stat, _ := os.Stat(localPath)
		if stat != nil {
			md.FileSize = stat.Size()
		}
	}
	if videoLimit > 0 && md.FileSize > 0 && bytesToMB(md.FileSize) > videoLimit {
		_ = w.storage.RemoveSafe(localPath)
		sizeMB := bytesToMB(md.FileSize)
		w.failWaiters(ctx, payload, tooLargeVideoText(st, videoLimit, sizeMB), nil)
		_ = w.media.UpdateDownloadMetrics(ctx, payload.DownloadID, nil, "FAILED", time.Since(payload.QueuedAt), time.Since(start), 0, 0, time.Since(start), "oversized")
		w.media.MarkDaily(ctx, payload.VariantType, false, "FAILED", true)
		w.locks.Release(ctx, lockKey)
		return nil
	}
	return w.queue.EnqueueSend(ctx, queue.SendTask{DownloadTask: payload, LocalPath: localPath, Metadata: md, DownloadMs: time.Since(start).Milliseconds()})
}

func (w *DownloadWorker) failWaiters(ctx context.Context, payload queue.DownloadTask, text string, technical error) {
	waiters, _ := w.locks.PopWaiters(ctx, queue.WaitersKey(payload.NormalizedURL, payload.VariantType, payload.Quality))
	if len(waiters) == 0 {
		waiters = []queue.Recipient{payload.Recipient}
	}
	for _, r := range waiters {
		msg := tgbotapi.NewMessage(r.ChatID, text)
		_, _ = w.bot.Send(msg)
		if r.DownloadID > 0 {
			_ = w.media.UpdateDownloadMetrics(ctx, r.DownloadID, nil, "FAILED", time.Since(payload.QueuedAt), 0, 0, 0, time.Since(payload.QueuedAt), fmt.Sprint(technical))
		}
		if technical != nil && w.logs != nil {
			userID := r.UserID
			w.logs.Write(ctx, &userID, "instagram", "worker_download", text, technical)
		}
	}
	if technical != nil {
		_ = w.queue.EnqueueNotification(ctx, queue.NotificationTask{Text: "Instagram download xatosi: " + technical.Error()})
	}
}

func knownSize(info downloader.ProbeInfo) int64 {
	if info.Filesize > 0 {
		return info.Filesize
	}
	return info.FilesizeApprox
}

func bytesToMB(v int64) int64 {
	if v <= 0 {
		return 0
	}
	return (v + 1024*1024 - 1) / (1024 * 1024)
}

func minPositive(a, b int64) int64 {
	if a <= 0 {
		return b
	}
	if b <= 0 {
		return a
	}
	if a < b {
		return a
	}
	return b
}

func effectiveUploadLimit(appLimit, telegramLimit int64, allowOversized bool) int64 {
	if allowOversized {
		return telegramLimit
	}
	return minPositive(appLimit, telegramLimit)
}

func telegramUploadLimit(st settings.Settings) int64 {
	if telegramMode(st.TelegramAPIMode) == "local" {
		if st.TelegramLocalMaxUploadMB > 0 {
			return st.TelegramLocalMaxUploadMB
		}
		return 2000
	}
	if st.TelegramCloudMaxUploadMB > 0 {
		return st.TelegramCloudMaxUploadMB
	}
	return 50
}

func telegramMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "local" {
		return "local"
	}
	return "cloud"
}

func tooLargeVideoText(st settings.Settings, limitMB, sizeMB int64) string {
	if telegramMode(st.TelegramAPIMode) == "cloud" {
		return telegram.CloudVideoTooLarge(limitMB, sizeMB)
	}
	return telegram.TooLargeVideo(limitMB, sizeMB)
}
