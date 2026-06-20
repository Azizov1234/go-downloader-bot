package downloader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"
)

type YTDLP struct {
	Bin string
}

type ProbeInfo struct {
	Title          string  `json:"title"`
	Filesize       int64   `json:"filesize"`
	FilesizeApprox int64   `json:"filesize_approx"`
	Duration       float64 `json:"duration"`
}

func (y YTDLP) Probe(ctx context.Context, rawURL, format string, cookiesArgs []string) (ProbeInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	args := []string{"--no-playlist", "--format", format, "--no-warnings", "--dump-json", "--skip-download"}
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	out, errOut, err := run(ctx, y.Bin, args...)
	if err != nil {
		return ProbeInfo{}, fmt.Errorf("yt-dlp probe failed: %w: %s", err, string(errOut))
	}
	var info ProbeInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return ProbeInfo{}, err
	}
	return info, nil
}

func (y YTDLP) Download(ctx context.Context, rawURL, format, outputDir, baseName string, cookiesArgs []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	template := filepath.Join(outputDir, baseName+".%(ext)s")
	args := []string{
		"--no-playlist",
		"--format", format,
		"--merge-output-format", "mp4",
		"--no-warnings",
		"-o", template,
	}
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	_, errOut, err := run(ctx, y.Bin, args...)
	if err != nil {
		return "", fmt.Errorf("yt-dlp download failed: %w: %s", err, string(errOut))
	}
	matches, err := filepath.Glob(filepath.Join(outputDir, baseName+".*"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("yt-dlp output not found")
	}
	return matches[0], nil
}

func run(ctx context.Context, bin string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}
