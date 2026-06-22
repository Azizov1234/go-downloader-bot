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
	cmd := exec.CommandContext(ctx, f.Bin,
		"-y",
		"-i", inputPath,
		"-vn",
		"-codec:a", "libmp3lame",
		"-q:a", "2",
		outputPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg mp3 failed: %w: %s", err, stderr.String())
	}
	return nil
}

// ImageToVideo creates an MP4 video from a static image + audio file.
// Uses -loop 1 to hold the image for the duration of the audio.
// Output is h264/aac progressive mp4 (faststart).
func (f FFMpeg) ImageToVideo(ctx context.Context, imagePath, audioPath, outputPath string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.Bin,
		"-y",
		"-loop", "1",
		"-i", imagePath,
		"-i", audioPath,
		"-shortest",
		"-c:v", "libx264",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "192k",
		"-movflags", "+faststart",
		outputPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg image-to-video failed: %w: %s", err, stderr.String())
	}
	return nil
}
