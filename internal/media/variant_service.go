package media

import "context"

type VariantService struct {
	media *Service
}

func NewVariantService(mediaService *Service) *VariantService {
	return &VariantService{media: mediaService}
}

func (s *VariantService) CacheKey(normalizedURL string, variantType VariantType, quality Quality) string {
	return CacheKey(normalizedURL, variantType, quality)
}

func (s *VariantService) Cached(ctx context.Context, normalizedURL string, variantType VariantType, quality Quality) (MediaVariant, error) {
	return s.media.FindCachedVariant(ctx, normalizedURL, variantType, quality)
}
