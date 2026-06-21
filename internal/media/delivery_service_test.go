package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"instagram-downloader-bot/internal/config"
)

type mockRoundTripper func(req *http.Request) (*http.Response, error)

func (m mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m(req)
}

func createTempFile(t *testing.T, name string, size int64) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, make([]byte, size), 0644)
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return path
}

func TestSendLocalTimed_LocalMode_Allowed(t *testing.T) {
	oldFunc := localFileSizeMBFunc
	defer func() { localFileSizeMBFunc = oldFunc }()
	localFileSizeMBFunc = func(path string) int64 {
		return 120
	}

	cfg := config.Config{
		TelegramAPIMode:                 "local",
		TelegramCloudMaxUploadMB:        50,
		TelegramLocalMaxUploadMB:        2000,
		TelegramUseLocalFilePath:        true,
		RequireLocalBotAPIForLargeFiles: true,
		TelegramLocalAPIURL:             "http://127.0.0.1:8081",
	}

	tempFilePath := createTempFile(t, "test_120mb.mp4", 100)

	httpCallCount := 0
	mockClient := &http.Client{
		Transport: mockRoundTripper(func(req *http.Request) (*http.Response, error) {
			httpCallCount++
			if strings.Contains(req.URL.Path, "getMe") {
				respStr := `{"ok":true,"result":{"id":123,"is_bot":true,"first_name":"FakeBot","username":"fake_bot"}}`
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(respStr)),
				}, nil
			}
			if strings.Contains(req.URL.Path, "sendVideo") {
				// Verify that it is indeed a multipart upload
				contentType := req.Header.Get("Content-Type")
				if !strings.HasPrefix(contentType, "multipart/form-data") {
					t.Errorf("expected Content-Type to start with 'multipart/form-data', got '%s'", contentType)
				}

				respStr := `{"ok":true,"result":{"message_id":123,"video":{"file_id":"fake_file_id","file_unique_id":"fake_uniq_id"}}}`
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(strings.NewReader(respStr)),
				}, nil
			}
			return nil, errors.New("unexpected request")
		}),
	}

	bot := &tgbotapi.BotAPI{}
	delivery := NewDeliveryService(bot, cfg, nil)
	delivery.http = mockClient

	variant := MediaVariant{
		VariantType: VariantVideo,
		Quality:     QualityAuto,
	}

	sent, err := delivery.SendLocalTimed(context.Background(), 12345, tempFilePath, variant, nil, 0)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if sent.FileID != "fake_file_id" {
		t.Errorf("expected file_id to be 'fake_file_id', got '%s'", sent.FileID)
	}

	if httpCallCount != 2 {
		t.Errorf("expected 2 http calls (getMe, sendVideo), got %d", httpCallCount)
	}
}

func TestSendLocalTimed_CloudMode_Rejected(t *testing.T) {
	oldFunc := localFileSizeMBFunc
	defer func() { localFileSizeMBFunc = oldFunc }()
	localFileSizeMBFunc = func(path string) int64 {
		return 120
	}

	cfg := config.Config{
		TelegramAPIMode:                 "cloud",
		TelegramCloudMaxUploadMB:        50,
		TelegramLocalMaxUploadMB:        2000,
		TelegramUseLocalFilePath:        false,
		RequireLocalBotAPIForLargeFiles: false,
	}

	tempFilePath := createTempFile(t, "test_120mb.mp4", 100)

	delivery := NewDeliveryService(&tgbotapi.BotAPI{}, cfg, nil)

	variant := MediaVariant{
		VariantType: VariantVideo,
		Quality:     QualityAuto,
	}

	_, err := delivery.SendLocalTimed(context.Background(), 12345, tempFilePath, variant, nil, 0)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, ErrTelegramFileTooLarge) {
		t.Errorf("expected ErrTelegramFileTooLarge error, got %v", err)
	}
}

func TestSendLocalTimed_LocalMode_Oversized(t *testing.T) {
	oldFunc := localFileSizeMBFunc
	defer func() { localFileSizeMBFunc = oldFunc }()
	localFileSizeMBFunc = func(path string) int64 {
		return 2100
	}

	cfg := config.Config{
		TelegramAPIMode:                 "local",
		TelegramCloudMaxUploadMB:        50,
		TelegramLocalMaxUploadMB:        2000,
		TelegramUseLocalFilePath:        true,
		RequireLocalBotAPIForLargeFiles: true,
	}

	tempFilePath := createTempFile(t, "test_2100mb.mp4", 100)

	delivery := NewDeliveryService(&tgbotapi.BotAPI{}, cfg, nil)

	variant := MediaVariant{
		VariantType: VariantVideo,
		Quality:     QualityAuto,
	}

	_, err := delivery.SendLocalTimed(context.Background(), 12345, tempFilePath, variant, nil, 0)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, ErrTelegramFileTooLarge) {
		t.Errorf("expected ErrTelegramFileTooLarge error, got %v", err)
	}

	var limitErr *TelegramLimitError
	if !errors.As(err, &limitErr) {
		t.Fatalf("expected error to be TelegramLimitError, got %v", err)
	}

	if limitErr.LimitMB != 2000 {
		t.Errorf("expected limit to be 2000 MB, got %d", limitErr.LimitMB)
	}
}
