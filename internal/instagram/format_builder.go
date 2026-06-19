package instagram

import (
	"instagram-downloader-bot/internal/media"
)

type FormatBuilder struct {
	formats map[string]string
}

func NewFormatBuilder(formats map[string]string) FormatBuilder {
	return FormatBuilder{formats: formats}
}

func (b FormatBuilder) For(variantType media.VariantType, quality media.Quality) string {
	if variantType == media.VariantAudio {
		if f := b.formats[string(media.QualityAuto)]; f != "" {
			return f
		}
		return "best[ext=mp4]/best"
	}
	if f := b.formats[string(quality)]; f != "" {
		return f
	}
	return b.formats[string(media.QualityAuto)]
}
