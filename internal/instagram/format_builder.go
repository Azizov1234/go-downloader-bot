package instagram

import (
	"instagram-downloader-bot/internal/media"
)

// Default format strings — optimized for speed:
// AUTO uses progressive mp4 up to 720p to avoid slow DASH merging.
// Quality-specific formats target exact height caps.
var defaultFormats = map[string]string{
	// AUTO: prefer progressive mp4 <=720p, fall back to best <=720p, then best
	string(media.QualityAuto): "bestvideo[ext=mp4][height<=720]+bestaudio[ext=m4a]/best[ext=mp4][height<=720]/best[height<=720]/best",

	// Specific resolutions
	string(media.QualityP1080): "bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/best[height<=1080][ext=mp4]/best[height<=1080]/best",
	string(media.QualityP720):  "bestvideo[height<=720][ext=mp4]+bestaudio[ext=m4a]/best[height<=720][ext=mp4]/best[height<=720]",
	string(media.QualityP480):  "bestvideo[height<=480][ext=mp4]+bestaudio[ext=m4a]/best[height<=480][ext=mp4]/best[height<=480]",

	// Audio only
	string(media.QualityMP3): "bestaudio[ext=m4a]/bestaudio",
}

type FormatBuilder struct {
	formats map[string]string
}

func NewFormatBuilder(formats map[string]string) FormatBuilder {
	// Merge provided formats over defaults
	merged := make(map[string]string, len(defaultFormats))
	for k, v := range defaultFormats {
		merged[k] = v
	}
	for k, v := range formats {
		if v != "" {
			merged[k] = v
		}
	}
	return FormatBuilder{formats: merged}
}

func (b FormatBuilder) For(variantType media.VariantType, quality media.Quality) string {
	if variantType == media.VariantAudio {
		if f := b.formats[string(media.QualityMP3)]; f != "" {
			return f
		}
		return "bestaudio[ext=m4a]/bestaudio"
	}
	if f := b.formats[string(quality)]; f != "" {
		return f
	}
	if f := b.formats[string(media.QualityAuto)]; f != "" {
		return f
	}
	return "bestvideo[ext=mp4][height<=720]+bestaudio[ext=m4a]/best[ext=mp4][height<=720]/best"
}
