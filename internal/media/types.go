package media

import "time"

type VariantType string
type Quality string

const (
	VariantVideo VariantType = "VIDEO"
	VariantAudio VariantType = "AUDIO"

	QualityAuto     Quality = "AUTO"
	QualityOriginal Quality = "ORIGINAL"
	QualityP1080    Quality = "P1080"
	QualityP720     Quality = "P720"
	QualityP480     Quality = "P480"
	QualitySmall    Quality = "SMALL"
	QualityMP3      Quality = "MP3"
)

type MediaFile struct {
	ID                 int64
	OriginalURL        string
	NormalizedURL      string
	InstagramShortcode string
	Platform           string
}

type MediaVariant struct {
	ID                   int64
	MediaFileID          int64
	NormalizedURL        string
	OriginalURL          string
	InstagramShortcode   string
	VariantType          VariantType
	Quality              Quality
	TelegramFileID       string
	TelegramFileUniqueID string
	Width                *int
	Height               *int
	Duration             *int
	FPS                  *float64
	Codec                *string
	FileSize             *int64
	Status               string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Metadata struct {
	Width    *int
	Height   *int
	Duration *int
	FPS      *float64
	Codec    *string
	FileSize int64
}

func CacheKey(normalizedURL string, variantType VariantType, quality Quality) string {
	return "instagram:" + normalizedURL + ":" + string(variantType) + ":" + string(quality)
}

func DisplayQuality(q Quality) string {
	switch q {
	case QualityAuto:
		return "Auto / Asl holati"
	case QualityOriginal:
		return "Original"
	case QualityP1080:
		return "1080p"
	case QualityP720:
		return "720p"
	case QualityP480:
		return "480p"
	case QualitySmall:
		return "Eng kichik hajm"
	case QualityMP3:
		return "MP3"
	default:
		return string(q)
	}
}
