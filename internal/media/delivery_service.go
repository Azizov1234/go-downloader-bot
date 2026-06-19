package media

import (
	"context"
	"fmt"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type DeliveryService struct {
	bot *tgbotapi.BotAPI
}

type SentFile struct {
	FileID       string
	FileUniqueID string
	MessageID    int
	SendDuration time.Duration
}

func NewDeliveryService(bot *tgbotapi.BotAPI) *DeliveryService {
	return &DeliveryService{bot: bot}
}

func (s *DeliveryService) SendByFileID(ctx context.Context, chatID int64, variant MediaVariant, replyMarkup any) (SentFile, error) {
	start := time.Now()
	var msg tgbotapi.Message
	var err error
	caption := Caption(variant)
	if variant.VariantType == VariantAudio {
		cfg := tgbotapi.NewAudio(chatID, tgbotapi.FileID(variant.TelegramFileID))
		cfg.Caption = caption
		cfg.ReplyMarkup = replyMarkup
		msg, err = s.bot.Send(cfg)
	} else {
		cfg := tgbotapi.NewVideo(chatID, tgbotapi.FileID(variant.TelegramFileID))
		cfg.Caption = caption
		cfg.SupportsStreaming = true
		cfg.ReplyMarkup = replyMarkup
		if variant.Duration != nil {
			cfg.Duration = *variant.Duration
		}
		msg, err = s.bot.Send(cfg)
	}
	if err != nil {
		return SentFile{}, err
	}
	return sentFromMessage(msg, variant.VariantType, time.Since(start))
}

func (s *DeliveryService) SendLocal(ctx context.Context, chatID int64, localPath string, variant MediaVariant, replyMarkup any) (SentFile, error) {
	start := time.Now()
	var msg tgbotapi.Message
	var err error
	caption := Caption(variant)
	if variant.VariantType == VariantAudio {
		cfg := tgbotapi.NewAudio(chatID, tgbotapi.FilePath(localPath))
		cfg.Caption = caption
		cfg.ReplyMarkup = replyMarkup
		msg, err = s.bot.Send(cfg)
	} else {
		cfg := tgbotapi.NewVideo(chatID, tgbotapi.FilePath(localPath))
		cfg.Caption = caption
		cfg.SupportsStreaming = true
		cfg.ReplyMarkup = replyMarkup
		if variant.Duration != nil {
			cfg.Duration = *variant.Duration
		}
		msg, err = s.bot.Send(cfg)
	}
	if err != nil {
		return SentFile{}, err
	}
	return sentFromMessage(msg, variant.VariantType, time.Since(start))
}

func Caption(v MediaVariant) string {
	return fmt.Sprintf("Instagram %s | %s", v.VariantType, DisplayQuality(v.Quality))
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
