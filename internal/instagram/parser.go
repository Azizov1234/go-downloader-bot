package instagram

import (
	"net/url"
	"strings"

	apperrors "instagram-downloader-bot/pkg/errors"
)

type ParsedURL struct {
	OriginalURL   string
	NormalizedURL string
	Shortcode     string
	Kind          string
}

func Parse(raw string) (ParsedURL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ParsedURL{}, apperrors.ErrInvalidURL
	}
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		trimmed = "https://" + trimmed
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Host == "" {
		return ParsedURL{}, apperrors.ErrInvalidURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ParsedURL{}, apperrors.ErrInvalidURL
	}
	host := strings.ToLower(u.Hostname())
	if !AllowedHost(host) {
		return ParsedURL{}, apperrors.ErrUnsupportedPlatform
	}
	return Normalize(trimmed)
}

func AllowedHost(host string) bool {
	switch strings.ToLower(host) {
	case "instagram.com", "www.instagram.com", "m.instagram.com":
		return true
	default:
		return false
	}
}
