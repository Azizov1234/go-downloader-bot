package queue

import (
	"time"

	"instagram-downloader-bot/internal/media"
)

const (
	TypeDownload     = "instagram:download"
	TypeSend         = "instagram:send"
	TypeAudioConvert = "instagram:audio_convert"
	TypeCleanup      = "cleanup:temp"
	TypeNotification = "notification:admin"

	QueuePrepare      = "instagram_prepare"
	QueueDownload     = "instagram_download"
	QueueSend         = "instagram_send"
	QueueAudioConvert = "instagram_audio_convert"
	QueueCleanup      = "cleanup"
	QueueNotification = "notification"
)

type Recipient struct {
	ChatID     int64  `json:"chat_id"`
	UserID     int64  `json:"user_id"`
	DownloadID int64  `json:"download_id"`
	Username   string `json:"username"`
}

type DownloadTask struct {
	Recipient          Recipient         `json:"recipient"`
	DownloadID         int64             `json:"download_id"`
	OriginalURL        string            `json:"original_url"`
	NormalizedURL      string            `json:"normalized_url"`
	InstagramShortcode string            `json:"instagram_shortcode"`
	VariantType        media.VariantType `json:"variant_type"`
	Quality            media.Quality     `json:"quality"`
	QueuedAt           time.Time         `json:"queued_at"`
}

type AudioConvertTask struct {
	DownloadTask
	SourcePath string `json:"source_path"`
	OutputPath string `json:"output_path"`
	DownloadMs int64  `json:"download_ms"`
}

type SendTask struct {
	DownloadTask
	LocalPath  string         `json:"local_path"`
	FileID     string         `json:"file_id"`
	UniqueID   string         `json:"unique_id"`
	Metadata   media.Metadata `json:"metadata"`
	DownloadMs int64          `json:"download_ms"`
	ConvertMs  int64          `json:"convert_ms"`
}

type NotificationTask struct {
	Text string `json:"text"`
}
