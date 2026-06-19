package donate

import (
	"context"
	"fmt"

	"instagram-downloader-bot/internal/settings"
)

type Service struct {
	settings *settings.Service
}

func NewService(settingsService *settings.Service) *Service {
	return &Service{settings: settingsService}
}

func (s *Service) Text(ctx context.Context) (string, error) {
	st, err := s.settings.Get(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s\n\nKarta: %s\nEgasi: %s", st.DonateText, st.DonateCardNumber, st.DonateCardOwner), nil
}
