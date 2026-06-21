package downloader

import (
	"context"
	"fmt"
	"log/slog"
)

type FallbackDownloader struct {
	YTDLP     YTDLP
	GalleryDL GalleryDL
	Logger    *slog.Logger
}

func NewFallbackDownloader(ytdlp YTDLP, galleryDL GalleryDL, logger *slog.Logger) FallbackDownloader {
	return FallbackDownloader{
		YTDLP:     ytdlp,
		GalleryDL: galleryDL,
		Logger:    logger,
	}
}

func (f FallbackDownloader) Probe(ctx context.Context, rawURL, format string, cookiesArgs []string) (ProbeInfo, error) {
	return f.YTDLP.Probe(ctx, rawURL, format, cookiesArgs)
}

func (f FallbackDownloader) Download(ctx context.Context, rawURL, format, outputDir, baseName string, cookiesArgs []string) (string, error) {
	path, err := f.YTDLP.Download(ctx, rawURL, format, outputDir, baseName, cookiesArgs)
	if err == nil {
		return path, nil
	}

	if f.Logger != nil {
		f.Logger.Warn("yt-dlp download failed, trying gallery-dl fallback", "error", err)
	}

	fallbackPath, fallbackErr := f.GalleryDL.Download(ctx, rawURL, format, outputDir, baseName, cookiesArgs)
	if fallbackErr == nil {
		if f.Logger != nil {
			f.Logger.Info("gallery-dl fallback download succeeded", "path", fallbackPath)
		}
		return fallbackPath, nil
	}

	return "", fmt.Errorf("yt-dlp failed (%w) and gallery-dl fallback failed (%v)", err, fallbackErr)
}
