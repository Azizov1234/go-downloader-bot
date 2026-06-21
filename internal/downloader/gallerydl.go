package downloader

import (
	"context"
	"fmt"
	"path/filepath"
	"time"
)

type GalleryDL struct {
	Bin string
}

func (g GalleryDL) Probe(ctx context.Context, rawURL, format string, cookiesArgs []string) (ProbeInfo, error) {
	return ProbeInfo{}, fmt.Errorf("gallery-dl probe not implemented")
}

func (g GalleryDL) Download(ctx context.Context, rawURL, format, outputDir, baseName string, cookiesArgs []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	var galleryCookies []string
	for i := 0; i < len(cookiesArgs); i++ {
		if cookiesArgs[i] == "--cookies" && i+1 < len(cookiesArgs) {
			galleryCookies = []string{"--cookies", cookiesArgs[i+1]}
			break
		}
	}

	args := []string{
		"--destination", outputDir,
		"--option", fmt.Sprintf("filename=%s.{extension}", baseName),
	}
	args = append(args, galleryCookies...)
	args = append(args, rawURL)

	_, errOut, err := run(ctx, g.Bin, args...)
	if err != nil {
		return "", fmt.Errorf("gallery-dl download failed: %w: %s", err, string(errOut))
	}

	matches, err := filepath.Glob(filepath.Join(outputDir, baseName+".*"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("gallery-dl output not found")
	}
	return matches[0], nil
}
