package downloader

import "context"

type Downloader interface {
	Probe(ctx context.Context, rawURL, format string, cookiesArgs []string) (ProbeInfo, error)
	Download(ctx context.Context, rawURL, format, outputDir, baseName string, cookiesArgs []string) (string, error)
}
