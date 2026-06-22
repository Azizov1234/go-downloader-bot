package downloader

import "context"

// Downloader is the interface for downloading media files.
type Downloader interface {
	Probe(ctx context.Context, rawURL, format string, cookiesArgs []string) (ProbeInfo, error)
	Download(ctx context.Context, rawURL, format, outputDir, baseName string, cookiesArgs []string) (string, error)
}

// RichProber can return full metadata including formats list and carousel entries.
// YTDLP implements this; FallbackDownloader wraps it.
type RichProber interface {
	ProbeRich(ctx context.Context, rawURL string, cookiesArgs []string) (RichProbeInfo, error)
	DownloadThumbnail(ctx context.Context, rawURL, outputDir, baseName string, cookiesArgs []string) (string, error)
	DownloadAudio(ctx context.Context, rawURL, outputDir, baseName string, cookiesArgs []string) (string, error)
}
