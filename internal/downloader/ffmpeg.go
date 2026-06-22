package downloader

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

type FFMpeg struct {
	Bin string
}

// ToMP3 converts an audio/video file to MP3 using libmp3lame.
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

// MergeVideoAudio merges separate video and audio streams into a single MP4 using
// stream copy (no re-encode). Falls back to re-encode merge if copy fails.
// Use this when yt-dlp downloads DASH streams that need muxing.
func (f FFMpeg) MergeVideoAudio(ctx context.Context, videoPath, audioPath, outputPath string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// First try: stream copy — fastest, no quality loss
	cmd := exec.CommandContext(ctx, f.Bin,
		"-y",
		"-i", videoPath,
		"-i", audioPath,
		"-c", "copy",
		"-movflags", "+faststart",
		outputPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		return outputPath, nil
	}

	// Fallback: re-encode audio only (video stays as-is with -c:v copy)
	stderr.Reset()
	fallbackOut := outputPath[:len(outputPath)-len(filepath.Ext(outputPath))] + "_fallback.mp4"
	cmd2 := exec.CommandContext(ctx, f.Bin,
		"-y",
		"-i", videoPath,
		"-i", audioPath,
		"-c:v", "copy",
		"-c:a", "aac",
		"-b:a", "192k",
		"-movflags", "+faststart",
		fallbackOut,
	)
	cmd2.Stderr = &stderr
	if err := cmd2.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg merge fallback failed: %w: %s", err, stderr.String())
	}
	return fallbackOut, nil
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
