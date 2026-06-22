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
	richProber     downloader.RichProber
	ffprobe        downloader.FFProbe
	ffmpeg         downloader.FFMpeg
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
	RichProber     downloader.RichProber
	FFProbe        downloader.FFProbe
	FFMpeg         downloader.FFMpeg
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
		bot: dep.Bot, logger: dep.Logger, downloader: dep.Downloader,
		richProber: dep.RichProber, ffprobe: dep.FFProbe, ffmpeg: dep.FFMpeg,
		formats: dep.Formats, cookies: dep.Cookies, storage: dep.Storage,
		media: dep.Media, settings: dep.Settings, users: dep.Users,
		logs: dep.Logs, queue: dep.Queue, locks: dep.Locks,
		allowOversized: dep.AllowOversized,
	}
}

func (w *DownloadWorker) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload queue.DownloadTask
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}

	totalStart := time.Now()

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

	st, err := w.settings.Get(ctx)
	if err != nil {
		w.failWaiters(ctx, payload, telegram.UniversalErrorMessage, err)
		w.locks.Release(ctx, lockKey)
		return err
	}

	// --- PROBE PHASE ---
	probeStart := time.Now()
	format := w.formats.For(payload.VariantType, payload.Quality)

	// Run ProbeRich exactly ONCE
	var richInfo downloader.RichProbeInfo
	var richErr error
	if w.richProber != nil {
		richInfo, richErr = w.richProber.ProbeRich(ctx, payload.OriginalURL, w.cookies.Args())
	}
	probeMs := time.Since(probeStart).Milliseconds()

	// If ProbeRich succeeds, check if it's image-only
	if richErr == nil && richInfo.IsImageOnly() {
		w.logger.Info("image-only post detected", "url", payload.OriginalURL, "probe_ms", probeMs)
		return w.processImagePost(ctx, payload, richInfo, st, lockKey, totalStart, probeMs)
	}

	// If ProbeRich itself failed with "no video formats" — treat as image
	if richErr != nil && downloader.IsVideoFormatNotFoundError(richErr) {
		w.logger.Info("no video formats found, trying image flow", "url", payload.OriginalURL, "probe_ms", probeMs, "err", richErr)
		return w.processImagePost(ctx, payload, richInfo, st, lockKey, totalStart, probeMs)
	}

	// --- NORMAL VIDEO FLOW ---
	var info downloader.ProbeInfo
	videoLimit := effectiveUploadLimit(st.MaxVideoFileSizeMB, telegramUploadLimit(st), w.allowOversized)
	var useGalleryDL bool
	var optFormat SelectedFormatInfo

	if richErr == nil {
		// ProbeRich succeeded, never call Probe again
		info = richInfo.ProbeInfo
		if payload.CustomTitle == "" && info.Title != "" {
			payload.CustomTitle = truncateTitle(info.Title, 100)
		}
		optFormat = selectFormat(richInfo.Formats, payload.Quality, format)
		format = optFormat.FormatID
		w.logger.Info("format selection log",
			"selected_format_id", optFormat.FormatID,
			"selected_resolution", optFormat.Resolution,
			"selected_filesize_mb", optFormat.FileSizeMB,
			"is_progressive", optFormat.IsProgressive,
			"needs_merge", optFormat.NeedsMerge,
		)
	} else {
		// ProbeRich failed, immediately fallback to gallery-dl (no yt-dlp retry, no basic Probe retry)
		w.logger.Warn("ProbeRich failed, immediately falling back to gallery-dl", "url", payload.OriginalURL, "error", richErr)
		useGalleryDL = true
		optFormat = SelectedFormatInfo{
			FormatID:      format,
			Resolution:    "unknown",
			FileSizeMB:    0,
			IsProgressive: false,
			NeedsMerge:    true,
		}
	}

	if !useGalleryDL && payload.VariantType == media.VariantVideo && videoLimit > 0 {
		size := knownSize(info)
		if size > 0 && bytesToMB(size) > videoLimit {
			sizeMB := bytesToMB(size)
			w.failWaiters(ctx, payload, tooLargeVideoText(st, videoLimit, sizeMB), nil)
			_ = w.media.UpdateDownloadMetrics(ctx, payload.DownloadID, nil, "FAILED", time.Since(payload.QueuedAt), 0, 0, 0, time.Since(totalStart), "oversized")
			w.media.MarkDaily(ctx, payload.VariantType, false, "FAILED", true)
			w.locks.Release(ctx, lockKey)
			return nil
		}
	}

	// --- DOWNLOAD PHASE ---
	dir, base, err := w.storage.DownloadBase(payload.Recipient.ChatID, payload.DownloadID, payload.Quality)
	if err != nil {
		w.failWaiters(ctx, payload, telegram.UniversalErrorMessage, err)
		w.locks.Release(ctx, lockKey)
		return err
	}
	downloadStart := time.Now()
	
	downloadCtx := ctx
	if useGalleryDL {
		downloadCtx = context.WithValue(ctx, downloader.SkipYTDLPKey, true)
	}
	localPath, err := w.downloader.Download(downloadCtx, payload.OriginalURL, format, dir, base, w.cookies.Args())
	downloadMs := time.Since(downloadStart).Milliseconds()
	if err != nil {
		// If download fails with no video formats -> try image flow
		if downloader.IsVideoFormatNotFoundError(err) {
			w.logger.Info("download no video formats, trying image flow",
				"url", payload.OriginalURL, "download_ms", downloadMs)
			w.locks.Release(ctx, lockKey)
			return w.processImagePost(ctx, payload, richInfo, st, lockKey, totalStart, time.Since(probeStart).Milliseconds())
		}
		w.logger.Error("download failed", "url", payload.OriginalURL,
			"download_ms", downloadMs, "total_ms", time.Since(totalStart).Milliseconds())
		w.failWaiters(ctx, payload, telegram.InstagramErrorMessage, err)
		_ = w.media.UpdateDownloadMetrics(ctx, payload.DownloadID, nil, "FAILED", time.Since(payload.QueuedAt), time.Duration(downloadMs)*time.Millisecond, 0, 0, time.Since(totalStart), err.Error())
		w.media.MarkDaily(ctx, payload.VariantType, false, "FAILED", false)
		w.locks.Release(ctx, lockKey)
		return nil
	}

	w.logger.Info("download complete",
		"url", payload.OriginalURL,
		"probe_ms", probeMs,
		"download_ms", downloadMs,
		"total_ms", time.Since(totalStart).Milliseconds(),
	)

	if payload.VariantType == media.VariantAudio {
		outPath, err := w.storage.AudioPath(payload.Recipient.ChatID, payload.DownloadID)
		if err != nil {
			_ = w.storage.RemoveSafe(localPath)
			w.failWaiters(ctx, payload, telegram.UniversalErrorMessage, err)
			w.locks.Release(ctx, lockKey)
			return err
		}
		return w.queue.EnqueueAudioConvert(ctx, queue.AudioConvertTask{
			DownloadTask: payload, SourcePath: localPath, OutputPath: outPath,
			ProbeMs:      probeMs,
			DownloadMs:   downloadMs,
		})
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
		_ = w.media.UpdateDownloadMetrics(ctx, payload.DownloadID, nil, "FAILED", time.Since(payload.QueuedAt), time.Duration(downloadMs)*time.Millisecond, 0, 0, time.Since(totalStart), "oversized")
		w.media.MarkDaily(ctx, payload.VariantType, false, "FAILED", true)
		w.locks.Release(ctx, lockKey)
		return nil
	}
	return w.queue.EnqueueSend(ctx, queue.SendTask{
		DownloadTask:   payload,
		LocalPath:      localPath,
		Metadata:       md,
		ProbeMs:        probeMs,
		DownloadMs:     downloadMs,
		MergeMs:        0,
		SelectedFormat: optFormat.FormatID,
		IsProgressive:  optFormat.IsProgressive,
		UsedFFmpeg:     optFormat.NeedsMerge,
		CacheHit:       false,
		Method:         "sendMultipartLocalAPI",
	})
}

// processImagePost handles image-only Instagram posts.
// Flow:
//  1. Download thumbnail (cover image)
//  2. Try to download audio track
//  3. If image + audio → ffmpeg → video MP4
//  4. If image only → send as photo
func (w *DownloadWorker) processImagePost(ctx context.Context, payload queue.DownloadTask, richInfo downloader.RichProbeInfo, st settings.Settings, lockKey string, totalStart time.Time, probeMs int64) error {
	if w.richProber == nil {
		w.failWaiters(ctx, payload, telegram.UniversalErrorMessage, fmt.Errorf("rich prober not available"))
		w.locks.Release(ctx, lockKey)
		return nil
	}

	dir, base, err := w.storage.DownloadBase(payload.Recipient.ChatID, payload.DownloadID, payload.Quality)
	if err != nil {
		w.failWaiters(ctx, payload, telegram.UniversalErrorMessage, err)
		w.locks.Release(ctx, lockKey)
		return err
	}

	// 1. Download thumbnail
	thumbStart := time.Now()
	imagePath, thumbErr := w.richProber.DownloadThumbnail(ctx, payload.OriginalURL, dir, base+"_thumb", w.cookies.Args())
	thumbMs := time.Since(thumbStart).Milliseconds()
	if thumbErr != nil {
		w.logger.Warn("thumbnail download failed", "url", payload.OriginalURL,
			"thumb_ms", thumbMs, "err", thumbErr)
		w.failWaiters(ctx, payload, telegram.InstagramErrorMessage, thumbErr)
		_ = w.media.UpdateDownloadMetrics(ctx, payload.DownloadID, nil, "FAILED", time.Since(payload.QueuedAt), 0, 0, 0, time.Since(totalStart), thumbErr.Error())
		w.media.MarkDaily(ctx, payload.VariantType, false, "FAILED", false)
		w.locks.Release(ctx, lockKey)
		return nil
	}
	defer func() { _ = w.storage.RemoveSafe(imagePath) }()

	w.logger.Info("thumbnail downloaded", "path", imagePath, "thumb_ms", thumbMs)

	// 2. Try to get audio
	audioStart := time.Now()
	audioPath, audioErr := w.richProber.DownloadAudio(ctx, payload.OriginalURL, dir, base+"_audio", w.cookies.Args())
	audioMs := time.Since(audioStart).Milliseconds()
	hasAudio := audioErr == nil && audioPath != ""

	var finalPath string
	var finalType media.VariantType
	var ffmpegMs int64

	if hasAudio {
		// 3. ffmpeg: image + audio → video
		defer func() { _ = w.storage.RemoveSafe(audioPath) }()
		outputPath := base + "_img2vid.mp4"
		outputPath = dir + "/" + outputPath

		ffmpegStart := time.Now()
		ffErr := w.ffmpeg.ImageToVideo(ctx, imagePath, audioPath, outputPath)
		ffmpegMs = time.Since(ffmpegStart).Milliseconds()

		if ffErr != nil {
			w.logger.Warn("ffmpeg image-to-video failed, sending image only",
				"url", payload.OriginalURL, "ffmpeg_ms", ffmpegMs, "err", ffErr)
			// Fall through to image-only send
			hasAudio = false
		} else {
			finalPath = outputPath
			finalType = media.VariantVideo
			w.logger.Info("ffmpeg image+audio → video done",
				"path", finalPath, "audio_ms", audioMs, "ffmpeg_ms", ffmpegMs)
		}
	}

	if !hasAudio {
		// 4. Send as photo
		finalPath = imagePath
		finalType = media.VariantImage
	}

	w.logger.Info("image post ready to send",
		"type", finalType, "path", finalPath,
		"thumb_ms", thumbMs, "audio_ms", audioMs, "ffmpeg_ms", ffmpegMs,
		"total_ms", time.Since(totalStart).Milliseconds(),
	)

	// Build a minimal variant for sending
	// We override the variant type so send_worker knows what to do
	imagePayload := payload
	imagePayload.VariantType = finalType
	titleText := ""
	if richInfo.ProbeInfo.Title != "" {
		titleText = truncateTitle(richInfo.ProbeInfo.Title, 100)
	}
	
	warningText := "Bu post video emas. Rasm bo‘lsa, rasm sifatida yuboraman."
	if titleText != "" {
		imagePayload.CustomTitle = warningText + "\n\n" + titleText
	} else {
		imagePayload.CustomTitle = warningText
	}

	if err := w.queue.EnqueueSend(ctx, queue.SendTask{
		DownloadTask:   imagePayload,
		LocalPath:      finalPath,
		Metadata:       media.Metadata{FileSize: 0}, // thumbnail size is small
		ProbeMs:        probeMs,
		DownloadMs:     thumbMs + audioMs,
		FFmpegMs:       ffmpegMs,
		MergeMs:        ffmpegMs,
		SelectedFormat: "image",
		IsProgressive:  !hasAudio,
		UsedFFmpeg:     hasAudio,
		CacheHit:       false,
		Method:         "sendMultipartLocalAPI",
	}); err != nil {
		w.failWaiters(ctx, payload, telegram.UniversalErrorMessage, err)
		w.locks.Release(ctx, lockKey)
		return nil
	}
	w.locks.Release(ctx, lockKey)
	return nil
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

func truncateTitle(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\t", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

type SelectedFormatInfo struct {
	FormatID      string
	Resolution    string
	FileSizeMB    float64
	IsProgressive bool
	NeedsMerge    bool
}

func selectFormat(formats []downloader.ProbeFormat, quality media.Quality, defaultFormat string) SelectedFormatInfo {
	if len(formats) == 0 {
		return SelectedFormatInfo{
			FormatID:      defaultFormat,
			Resolution:    "unknown",
			FileSizeMB:    0,
			IsProgressive: false,
			NeedsMerge:    true,
		}
	}

	findBestAudio := func() *downloader.ProbeFormat {
		var bestAudio *downloader.ProbeFormat
		// First pass: look for audio-only streams (no video)
		for i := range formats {
			f := &formats[i]
			if f.Acodec != "" && f.Acodec != "none" && (f.Vcodec == "" || f.Vcodec == "none") {
				if bestAudio == nil || f.Filesize > bestAudio.Filesize {
					bestAudio = f
				}
			}
		}
		// Second pass fallback: any stream with audio if no audio-only exists
		if bestAudio == nil {
			for i := range formats {
				f := &formats[i]
				if f.Acodec != "" && f.Acodec != "none" {
					if bestAudio == nil || f.Filesize > bestAudio.Filesize {
						bestAudio = f
					}
				}
			}
		}
		return bestAudio
	}

	// 1. AUTO: progressive MP4 priority, else DASH bestvideo+bestaudio
	if quality == media.QualityAuto {
		var bestProg *downloader.ProbeFormat
		for i := range formats {
			f := &formats[i]
			if f.Vcodec != "" && f.Vcodec != "none" &&
				f.Acodec != "" && f.Acodec != "none" &&
				f.Ext == "mp4" {
				if bestProg == nil || f.Height > bestProg.Height || (f.Height == bestProg.Height && f.Filesize > bestProg.Filesize) {
					bestProg = f
				}
			}
		}
		if bestProg != nil {
			return SelectedFormatInfo{
				FormatID:      bestProg.FormatID,
				Resolution:    fmt.Sprintf("%dx%d", bestProg.Width, bestProg.Height),
				FileSizeMB:    float64(bestProg.Filesize) / (1024 * 1024),
				IsProgressive: true,
				NeedsMerge:    false,
			}
		}

		var bestVideo *downloader.ProbeFormat
		for i := range formats {
			f := &formats[i]
			if f.Vcodec != "" && f.Vcodec != "none" {
				if bestVideo == nil || f.Height > bestVideo.Height || (f.Height == bestVideo.Height && f.Filesize > bestVideo.Filesize) {
					bestVideo = f
				}
			}
		}
		bestAudio := findBestAudio()
		if bestVideo != nil && bestAudio != nil {
			formatID := bestVideo.FormatID + "+" + bestAudio.FormatID
			needsMerge := true
			isProgressive := false
			if bestVideo.Acodec != "" && bestVideo.Acodec != "none" {
				formatID = bestVideo.FormatID
				needsMerge = false
				isProgressive = true
			}
			totalSize := bestVideo.Filesize
			if needsMerge {
				totalSize += bestAudio.Filesize
			}
			return SelectedFormatInfo{
				FormatID:      formatID,
				Resolution:    fmt.Sprintf("%dx%d", bestVideo.Width, bestVideo.Height),
				FileSizeMB:    float64(totalSize) / (1024 * 1024),
				IsProgressive: isProgressive,
				NeedsMerge:    needsMerge,
			}
		}
	}

	// 2. ORIGINAL: highest possible format
	if quality == media.QualityOriginal {
		var bestVideo *downloader.ProbeFormat
		for i := range formats {
			f := &formats[i]
			if f.Vcodec != "" && f.Vcodec != "none" {
				if bestVideo == nil || f.Height > bestVideo.Height || (f.Height == bestVideo.Height && f.Filesize > bestVideo.Filesize) {
					bestVideo = f
				}
			}
		}
		bestAudio := findBestAudio()
		if bestVideo != nil && bestAudio != nil {
			formatID := bestVideo.FormatID + "+" + bestAudio.FormatID
			needsMerge := true
			isProgressive := false
			if bestVideo.Acodec != "" && bestVideo.Acodec != "none" {
				formatID = bestVideo.FormatID
				needsMerge = false
				isProgressive = true
			}
			totalSize := bestVideo.Filesize
			if needsMerge {
				totalSize += bestAudio.Filesize
			}
			return SelectedFormatInfo{
				FormatID:      formatID,
				Resolution:    fmt.Sprintf("%dx%d", bestVideo.Width, bestVideo.Height),
				FileSizeMB:    float64(totalSize) / (1024 * 1024),
				IsProgressive: isProgressive,
				NeedsMerge:    needsMerge,
			}
		}
	}

	// 3. Specific resolution capped (1080p, 720p, 480p)
	var targetHeight int
	switch quality {
	case media.QualityP1080:
		targetHeight = 1080
	case media.QualityP720:
		targetHeight = 720
	case media.QualityP480:
		targetHeight = 480
	}
	if targetHeight > 0 {
		var bestVideo *downloader.ProbeFormat
		for i := range formats {
			f := &formats[i]
			if f.Vcodec != "" && f.Vcodec != "none" && f.Height <= targetHeight {
				if bestVideo == nil || f.Height > bestVideo.Height || (f.Height == bestVideo.Height && f.Filesize > bestVideo.Filesize) {
					bestVideo = f
				}
			}
		}
		bestAudio := findBestAudio()
		if bestVideo != nil && bestAudio != nil {
			formatID := bestVideo.FormatID + "+" + bestAudio.FormatID
			needsMerge := true
			isProgressive := false
			if bestVideo.Acodec != "" && bestVideo.Acodec != "none" {
				formatID = bestVideo.FormatID
				needsMerge = false
				isProgressive = true
			}
			totalSize := bestVideo.Filesize
			if needsMerge {
				totalSize += bestAudio.Filesize
			}
			return SelectedFormatInfo{
				FormatID:      formatID,
				Resolution:    fmt.Sprintf("%dx%d", bestVideo.Width, bestVideo.Height),
				FileSizeMB:    float64(totalSize) / (1024 * 1024),
				IsProgressive: isProgressive,
				NeedsMerge:    needsMerge,
			}
		}
	}

	return SelectedFormatInfo{
		FormatID:      defaultFormat,
		Resolution:    "unknown",
		FileSizeMB:    0,
		IsProgressive: false,
		NeedsMerge:    true,
	}
}
