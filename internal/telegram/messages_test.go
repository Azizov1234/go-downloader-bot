package telegram

import (
	"strings"
	"testing"
)

func TestTelegramUploadTooLarge_LocalMode(t *testing.T) {
	msg := TelegramUploadTooLarge("local", 2000, 148)
	if !strings.Contains(msg, "Local Bot API limiti: 2000 MB") {
		t.Errorf("expected message to mention 'Local Bot API limiti: 2000 MB', got:\n%s", msg)
	}
	if strings.Contains(msg, "Cloud Bot API limiti") {
		t.Errorf("expected message not to mention 'Cloud Bot API limiti', got:\n%s", msg)
	}
}

func TestTelegramUploadTooLarge_CloudMode(t *testing.T) {
	msg := TelegramUploadTooLarge("cloud", 50, 148)
	if !strings.Contains(msg, "Cloud Bot API limiti: 50 MB") {
		t.Errorf("expected message to mention 'Cloud Bot API limiti: 50 MB', got:\n%s", msg)
	}
}

func TestLocalBotAPIUnavailable(t *testing.T) {
	msg := LocalBotAPIUnavailable("http://127.0.0.1:8081")
	if !strings.Contains(msg, "Local Telegram Bot API ishlamayapti yoki 127.0.0.1:8081 ulanmayapti") {
		t.Errorf("expected message to contain dynamic host, got:\n%s", msg)
	}
}
