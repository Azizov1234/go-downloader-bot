package instagram

import (
	"net/url"
	"path"
	"strings"

	apperrors "instagram-downloader-bot/pkg/errors"
)

func Normalize(raw string) (ParsedURL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ParsedURL{}, apperrors.ErrInvalidURL
	}
	host := strings.ToLower(u.Hostname())
	if !AllowedHost(host) {
		return ParsedURL{}, apperrors.ErrUnsupportedPlatform
	}

	parts := cleanParts(u.EscapedPath())
	if len(parts) < 2 {
		return ParsedURL{}, apperrors.ErrUnsupportedPlatform
	}

	kind := strings.ToLower(parts[0])
	switch kind {
	case "p", "reel", "tv":
		shortcode := parts[1]
		normalized := "https://www.instagram.com/" + kind + "/" + shortcode + "/"
		return ParsedURL{OriginalURL: raw, NormalizedURL: normalized, Shortcode: shortcode, Kind: kind}, nil
	case "stories":
		if len(parts) < 3 {
			return ParsedURL{}, apperrors.ErrUnsupportedPlatform
		}
		storyUser := parts[1]
		storyID := parts[2]
		normalized := "https://www.instagram.com/stories/" + storyUser + "/" + storyID + "/"
		return ParsedURL{OriginalURL: raw, NormalizedURL: normalized, Shortcode: storyUser + "-" + storyID, Kind: kind}, nil
	default:
		return ParsedURL{}, apperrors.ErrUnsupportedPlatform
	}
}

func cleanParts(escapedPath string) []string {
	p := path.Clean("/" + escapedPath)
	if p == "/" {
		return nil
	}
	raw := strings.Split(strings.Trim(p, "/"), "/")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
