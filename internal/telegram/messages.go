package telegram

import (
	"fmt"
	"net/url"
	"strings"

	"instagram-downloader-bot/internal/saved"
	"instagram-downloader-bot/internal/users"
)

const (
	UnsupportedPlatformMessage = "Hozircha faqat Instagram linklari qo'llab-quvvatlanadi."
	InstagramErrorMessage      = "Media yuklab bo‘lmadi. Link private, o‘chirilgan yoki platforma cheklovi bo‘lishi mumkin."
	UniversalErrorMessage      = "Media yuklab bo‘lmadi. Link private, o‘chirilgan yoki platforma cheklovi bo‘lishi mumkin."
	SendFailedMessage          = "Media yuklandi, lekin Telegramga yuborishda xatolik bo'ldi."
)

func TooLargeVideo(limitMB int64, sizeMB int64) string {
	return fmt.Sprintf("Video hajmi juda katta.\n\nLocal Bot API limiti: %d MB\nVideo hajmi: %d MB", limitMB, sizeMB)
}

func CloudVideoTooLarge(limitMB int64, sizeMB int64) string {
	return fmt.Sprintf("Video hajmi juda katta.\n\nCloud Bot API limiti: %d MB\nVideo hajmi: %d MB\n\n2GB gacha video yuborish uchun Local Telegram Bot API Server kerak.", limitMB, sizeMB)
}

func TooLargeAudio(limitMB int64, sizeMB int64) string {
	return fmt.Sprintf("Audio hajmi juda katta.\n\nAdmin belgilagan limit: %d MB\nAudio hajmi: %d MB", limitMB, sizeMB)
}

func TelegramUploadTooLarge(mode string, limitMB, sizeMB int64) string {
	if mode == "local" {
		return TooLargeVideo(limitMB, sizeMB)
	}
	return CloudVideoTooLarge(limitMB, sizeMB)
}

func LocalBotAPIUnavailable(apiURL string) string {
	host := "127.0.0.1:8081"
	if apiURL != "" {
		if u, err := url.Parse(apiURL); err == nil {
			host = u.Host
		}
	}
	return fmt.Sprintf("Local Telegram Bot API ishlamayapti yoki %s ulanmayapti", host)
}

func ProfileText(p users.Profile) string {
	last := "yo'q"
	if p.LastDownloadAt != nil {
		last = p.LastDownloadAt.Format("2006-01-02 15:04")
	}
	return fmt.Sprintf("Profil\n\nTelegram ID: %d\nUsername: @%s\nJami download: %d\nMuvaffaqiyatli: %d\nFailed: %d\nSaqlanganlar: %d\nBugungi download: %d\nOxirgi download: %s",
		p.TelegramID, p.Username, p.DownloadsCount, p.SuccessDownloads, p.FailedDownloads, p.SavedCount, p.TodayDownloads, last)
}

func SavedListText(items []saved.Item, page, total int) string {
	if total == 0 {
		return "Saqlangan media hali yo'q."
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Saqlanganlar (%d ta)\n\n", total))
	for _, item := range items {
		b.WriteString(fmt.Sprintf("%s | %s | %s | %s\n%s\n\n",
			saved.Number(item.SaveNumber), item.Platform, item.VariantType, item.Quality, item.CreatedAt.Format("2006-01-02 15:04")))
	}
	b.WriteString(fmt.Sprintf("Sahifa: %d", page))
	return b.String()
}
