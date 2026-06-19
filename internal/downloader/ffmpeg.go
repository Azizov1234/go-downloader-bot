package downloader

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

type FFMpeg struct {
	Bin string
}

func (f FFMpeg) ToMP3(ctx context.Context, inputPath, outputPath string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.Bin, "-y", "-i", inputPath, "-vn", "-codec:a", "libmp3lame", "-q:a", "2", outputPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg mp3 failed: %w: %s", err, stderr.String())
	}
	return nil
}
