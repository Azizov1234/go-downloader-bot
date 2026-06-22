package instagram

import (
	"instagram-downloader-bot/internal/media"
)

// Default format strings — speed-optimized:
//
// AUTO:
//   - Sifatni pasaytirmaydi — original yuqori sifatni oladi
//   - Progressive mp4 mavjud bo'lsa priority — merge kerak bo'lmaydi (tez)
//   - Bo'lmasa DASH bestvideo+bestaudio merge qiladi (ffmpeg -c copy)
//   - Eng so'nggi fallback: "best" (birlashtirilgan)
//
// ORIGINAL:
//   - Hamma vaqt eng yuqori sifat, hech qanday cheklovsiz
//
// Specific resolutions:
//   - Faqat height cap bor, ext+merge priority
var defaultFormats = map[string]string{
	// AUTO: progressive mp4 priority (merge yo'q → tez), keyin DASH merge, keyin best
	string(media.QualityAuto): "bestvideo[ext=mp4]+bestaudio[ext=m4a]/bestvideo+bestaudio/best[ext=mp4]/best",

	// ORIGINAL: hech qanday cheklov yo'q — eng yuqori sifat
	string(media.QualityOriginal): "bestvideo+bestaudio/best",

	// Specific resolutions — progressive priority, keyin DASH, keyin height-capped best
	string(media.QualityP1080): "bestvideo[height<=1080][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=1080]+bestaudio/best[height<=1080]",
	string(media.QualityP720):  "bestvideo[height<=720][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=720]+bestaudio/best[height<=720]",
	string(media.QualityP480):  "bestvideo[height<=480][ext=mp4]+bestaudio[ext=m4a]/bestvideo[height<=480]+bestaudio/best[height<=480]",

	// Audio only
	string(media.QualityMP3): "bestaudio[ext=m4a]/bestaudio",
}

type FormatBuilder struct {
	formats map[string]string
}

func NewFormatBuilder(formats map[string]string) FormatBuilder {
	// Merge user-provided formats over defaults (user overrides win)
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
	return "bestvideo[ext=mp4]+bestaudio[ext=m4a]/bestvideo+bestaudio/best[ext=mp4]/best"
}
