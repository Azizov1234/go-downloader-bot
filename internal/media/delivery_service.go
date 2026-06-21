package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"instagram-downloader-bot/internal/config"
	"instagram-downloader-bot/internal/settings"
)

var (
	ErrTelegramFileTooLarge     = errors.New("telegram file too large")
	ErrLocalBotAPIUnavailable   = errors.New("local telegram bot api unavailable")
	ErrLocalBotAPIConfiguration = errors.New("local telegram bot api is not configured")
)

var localFileSizeMBFunc = func(path string) int64 {
	stat, err := os.Stat(path)
	if err != nil || stat == nil {
		return 0
	}
	return bytesToMegabytes(stat.Size())
}

type TelegramLimitError struct {
	Mode    string
	LimitMB int64
	SizeMB  int64
}

func (e *TelegramLimitError) Error() string {
	return fmt.Sprintf("%s telegram upload limit exceeded: limit=%d MB size=%d MB", e.Mode, e.LimitMB, e.SizeMB)
}

func (e *TelegramLimitError) Unwrap() error {
	return ErrTelegramFileTooLarge
}

type LocalBotAPIError struct {
	URL string
	Err error
}

func (e *LocalBotAPIError) Error() string {
	if e.Err == nil {
		return "local telegram bot api unavailable"
	}
	return fmt.Sprintf("local telegram bot api unavailable at %s: %v", e.URL, e.Err)
}

func (e *LocalBotAPIError) Unwrap() error {
	return ErrLocalBotAPIUnavailable
}

type DeliveryService struct {
	bot      *tgbotapi.BotAPI
	cfg      config.Config
	settings *settings.Service
	http     *http.Client
}

type SentFile struct {
	FileID       string
	FileUniqueID string
	MessageID    int
	SendDuration time.Duration
}

func NewDeliveryService(bot *tgbotapi.BotAPI, cfg config.Config, settingsService *settings.Service) *DeliveryService {
	return &DeliveryService{
		bot:      bot,
		cfg:      cfg,
		settings: settingsService,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *DeliveryService) SendByFileID(ctx context.Context, chatID int64, variant MediaVariant, replyMarkup any) (SentFile, error) {
	return s.SendByFileIDTimed(ctx, chatID, variant, replyMarkup, 0)
}

func (s *DeliveryService) SendByFileIDTimed(ctx context.Context, chatID int64, variant MediaVariant, replyMarkup any, elapsed time.Duration) (SentFile, error) {
	start := time.Now()
	caption := CaptionWithElapsed(variant, elapsed)
	msg, err := s.sendFileIDViaCloud(ctx, chatID, variant, caption, replyMarkup)
	if err != nil {
		return SentFile{}, err
	}
	return sentFromMessage(msg, variant.VariantType, time.Since(start))
}

func (s *DeliveryService) SendLocal(ctx context.Context, chatID int64, localPath string, variant MediaVariant, replyMarkup any) (SentFile, error) {
	return s.SendLocalTimed(ctx, chatID, localPath, variant, replyMarkup, 0)
}

func (s *DeliveryService) SendLocalTimed(ctx context.Context, chatID int64, localPath string, variant MediaVariant, replyMarkup any, elapsed time.Duration) (SentFile, error) {
	start := time.Now()
	st, err := s.CurrentSettings(ctx)
	if err != nil {
		return SentFile{}, err
	}
	mode := telegramMode(st.TelegramAPIMode)
	sizeMB := localFileSizeMBFunc(localPath)
	limitMB := uploadLimitForMode(st, mode)
	if limitMB > 0 && sizeMB > limitMB {
		return SentFile{}, &TelegramLimitError{Mode: mode, LimitMB: limitMB, SizeMB: sizeMB}
	}

	caption := CaptionWithElapsed(variant, elapsed)
	var msg tgbotapi.Message
	method := "sendCloudUpload"
	if mode == "local" {
		method = "sendLocalPath"
	}

	log.Printf("send local media mode=%s size_mb=%d effective_limit_mb=%d local_api_url=%s method=%s",
		mode, sizeMB, limitMB, s.cfg.TelegramLocalAPIURL, method)

	if mode == "local" {
		isOverCloudLimit := sizeMB > st.TelegramCloudMaxUploadMB
		if isOverCloudLimit {
			if healthErr := s.HealthCheck(ctx); healthErr != nil {
				return SentFile{}, &LocalBotAPIError{URL: s.cfg.TelegramLocalAPIURL, Err: healthErr}
			}
		}

		msg, err = s.sendLocalPath(ctx, chatID, localPath, variant, caption, replyMarkup)
		if err != nil && !isOverCloudLimit {
			// Fallback to cloud only for small files
			msg, err = s.sendCloudUpload(chatID, localPath, variant, caption, replyMarkup)
		}
	} else {
		msg, err = s.sendCloudUpload(chatID, localPath, variant, caption, replyMarkup)
	}
	if err != nil {
		return SentFile{}, err
	}
	return sentFromMessage(msg, variant.VariantType, time.Since(start))
}

func (s *DeliveryService) HealthCheck(ctx context.Context) error {
	if strings.TrimSpace(s.cfg.TelegramLocalAPIURL) == "" {
		return ErrLocalBotAPIConfiguration
	}
	_, err := s.localRequest(ctx, "getMe", url.Values{})
	return err
}

func (s *DeliveryService) Config() config.Config {
	return s.cfg
}

func (s *DeliveryService) CurrentSettings(ctx context.Context) (settings.Settings, error) {
	if s.settings != nil {
		st, err := s.settings.Get(ctx)
		if err == nil {
			return st, nil
		}
	}
	return settings.Settings{
		TelegramAPIMode:                 s.cfg.TelegramAPIMode,
		TelegramCloudMaxUploadMB:        s.cfg.TelegramCloudMaxUploadMB,
		TelegramLocalMaxUploadMB:        s.cfg.TelegramLocalMaxUploadMB,
		TelegramUseLocalFilePath:        s.cfg.TelegramUseLocalFilePath,
		RequireLocalBotAPIForLargeFiles: s.cfg.RequireLocalBotAPIForLargeFiles,
		TelegramMaxUploadMB:             s.cfg.TelegramMaxUploadMB,
		MaxVideoFileSizeMB:              s.cfg.MaxVideoFileSizeMB,
		MaxAudioFileSizeMB:              s.cfg.MaxAudioFileSizeMB,
	}, nil
}

func (s *DeliveryService) sendCloudUpload(chatID int64, localPath string, variant MediaVariant, caption string, replyMarkup any) (tgbotapi.Message, error) {
	if variant.VariantType == VariantAudio {
		cfg := tgbotapi.NewAudio(chatID, tgbotapi.FilePath(localPath))
		cfg.Caption = caption
		cfg.ReplyMarkup = replyMarkup
		return s.bot.Send(cfg)
	}
	cfg := tgbotapi.NewVideo(chatID, tgbotapi.FilePath(localPath))
	cfg.Caption = caption
	cfg.SupportsStreaming = true
	cfg.ReplyMarkup = replyMarkup
	if variant.Duration != nil {
		cfg.Duration = *variant.Duration
	}
	return s.bot.Send(cfg)
}

func (s *DeliveryService) sendFileIDViaCloud(ctx context.Context, chatID int64, variant MediaVariant, caption string, replyMarkup any) (tgbotapi.Message, error) {
	values := url.Values{}
	values.Set("chat_id", strconv.FormatInt(chatID, 10))
	values.Set("caption", caption)
	addReplyMarkup(values, replyMarkup)
	method := "sendVideo"
	if variant.VariantType == VariantAudio {
		method = "sendAudio"
		values.Set("audio", variant.TelegramFileID)
	} else {
		values.Set("video", variant.TelegramFileID)
		values.Set("supports_streaming", "true")
		if variant.Duration != nil {
			values.Set("duration", strconv.Itoa(*variant.Duration))
		}
	}
	return s.cloudRequest(ctx, method, values)
}

func (s *DeliveryService) sendLocalPath(ctx context.Context, chatID int64, localPath string, variant MediaVariant, caption string, replyMarkup any) (tgbotapi.Message, error) {
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return tgbotapi.Message{}, err
	}
	values := url.Values{}
	values.Set("chat_id", strconv.FormatInt(chatID, 10))
	values.Set("caption", caption)
	addReplyMarkup(values, replyMarkup)
	method := "sendVideo"
	if variant.VariantType == VariantAudio {
		method = "sendAudio"
		values.Set("audio", absPath)
	} else {
		values.Set("video", absPath)
		values.Set("supports_streaming", "true")
		if variant.Duration != nil {
			values.Set("duration", strconv.Itoa(*variant.Duration))
		}
	}
	return s.localRequest(ctx, method, values)
}

func (s *DeliveryService) cloudRequest(ctx context.Context, method string, values url.Values) (tgbotapi.Message, error) {
	return s.request(ctx, apiMethodURL(s.cfg.TelegramCloudAPIURL, s.cfg.BotToken, method), method, values, false)
}

func (s *DeliveryService) localRequest(ctx context.Context, method string, values url.Values) (tgbotapi.Message, error) {
	return s.request(ctx, apiMethodURL(s.cfg.TelegramLocalAPIURL, s.cfg.BotToken, method), method, values, true)
}

func (s *DeliveryService) request(ctx context.Context, endpoint, method string, values url.Values, local bool) (tgbotapi.Message, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return tgbotapi.Message{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.http.Do(req)
	if err != nil {
		if local {
			return tgbotapi.Message{}, &LocalBotAPIError{URL: s.cfg.TelegramLocalAPIURL, Err: err}
		}
		return tgbotapi.Message{}, err
	}
	defer resp.Body.Close()

	var apiResp tgbotapi.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		if local {
			return tgbotapi.Message{}, &LocalBotAPIError{URL: s.cfg.TelegramLocalAPIURL, Err: err}
		}
		return tgbotapi.Message{}, err
	}
	if !apiResp.Ok {
		err := fmt.Errorf("telegram %s failed: %s", method, apiResp.Description)
		if local && resp.StatusCode >= 500 {
			return tgbotapi.Message{}, &LocalBotAPIError{URL: s.cfg.TelegramLocalAPIURL, Err: err}
		}
		return tgbotapi.Message{}, err
	}
	if len(apiResp.Result) == 0 {
		return tgbotapi.Message{}, nil
	}
	var msg tgbotapi.Message
	if err := json.Unmarshal(apiResp.Result, &msg); err != nil {
		return tgbotapi.Message{}, err
	}
	return msg, nil
}

func Caption(v MediaVariant) string {
	return fmt.Sprintf("Instagram %s | %s", v.VariantType, DisplayQuality(v.Quality))
}

func CaptionWithElapsed(v MediaVariant, elapsed time.Duration) string {
	caption := Caption(v)
	if elapsed <= 0 {
		return caption
	}
	return fmt.Sprintf("%s\n%.1f sekundda keldi", caption, elapsed.Seconds())
}

func sentFromMessage(msg tgbotapi.Message, variantType VariantType, took time.Duration) (SentFile, error) {
	out := SentFile{MessageID: msg.MessageID, SendDuration: took}
	if variantType == VariantAudio {
		if msg.Audio == nil {
			return out, fmt.Errorf("telegram audio response has no file")
		}
		out.FileID = msg.Audio.FileID
		out.FileUniqueID = msg.Audio.FileUniqueID
		return out, nil
	}
	if msg.Video == nil {
		return out, fmt.Errorf("telegram video response has no file")
	}
	out.FileID = msg.Video.FileID
	out.FileUniqueID = msg.Video.FileUniqueID
	return out, nil
}

func IsTelegramFileTooLarge(err error) bool {
	return errors.Is(err, ErrTelegramFileTooLarge)
}

func IsLocalBotAPIUnavailable(err error) bool {
	return errors.Is(err, ErrLocalBotAPIUnavailable)
}

func TelegramLimit(err error) (limitMB, sizeMB int64, ok bool) {
	var limitErr *TelegramLimitError
	if errors.As(err, &limitErr) {
		return limitErr.LimitMB, limitErr.SizeMB, true
	}
	return 0, 0, false
}

func addReplyMarkup(values url.Values, replyMarkup any) {
	if replyMarkup == nil {
		return
	}
	body, err := json.Marshal(replyMarkup)
	if err == nil && len(body) > 0 && string(body) != "null" {
		values.Set("reply_markup", string(body))
	}
}

func apiMethodURL(baseURL, token, method string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	return baseURL + "/bot" + token + "/" + method
}

func telegramMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "local" {
		return "local"
	}
	return "cloud"
}

func uploadLimitForMode(st settings.Settings, mode string) int64 {
	if mode == "local" {
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

func localFileSizeMB(path string) int64 {
	stat, err := os.Stat(path)
	if err != nil || stat == nil {
		return 0
	}
	return bytesToMegabytes(stat.Size())
}

func bytesToMegabytes(v int64) int64 {
	if v <= 0 {
		return 0
	}
	return (v + 1024*1024 - 1) / (1024 * 1024)
}
