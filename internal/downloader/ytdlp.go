package downloader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type YTDLP struct {
	Bin string
}

// ProbeInfo — basic info from yt-dlp --dump-json (fast, single-stream)
type ProbeInfo struct {
	Title          string  `json:"title"`
	Filesize       int64   `json:"filesize"`
	FilesizeApprox int64   `json:"filesize_approx"`
	Duration       float64 `json:"duration"`
	Ext            string  `json:"ext"`
	Thumbnail      string  `json:"thumbnail"`
	// _type is "video", "playlist", etc.
	Type string `json:"_type"`
}

// RichProbeInfo — detailed info including formats list and carousel entries
type RichProbeInfo struct {
	ProbeInfo
	Formats  []ProbeFormat  `json:"formats"`
	Entries  []ProbeEntry   `json:"entries"`
	MusicURL string         `json:"music_url,omitempty"`
}

type ProbeFormat struct {
	FormatID string `json:"format_id"`
	Ext      string `json:"ext"`
	Vcodec   string `json:"vcodec"`
	Acodec   string `json:"acodec"`
	Height   int    `json:"height"`
	Width    int    `json:"width"`
	Filesize int64  `json:"filesize"`
}

type ProbeEntry struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	Ext       string  `json:"ext"`
	Thumbnail string  `json:"thumbnail"`
	Duration  float64 `json:"duration"`
	Formats   []ProbeFormat `json:"formats"`
}

// HasVideoFormats returns true if the probe result has at least one real video format
func (r *RichProbeInfo) HasVideoFormats() bool {
	for _, f := range r.Formats {
		if f.Vcodec != "" && f.Vcodec != "none" && f.Height > 0 {
			return true
		}
	}
	// entries-based carousel
	for _, e := range r.Entries {
		for _, f := range e.Formats {
			if f.Vcodec != "" && f.Vcodec != "none" && f.Height > 0 {
				return true
			}
		}
	}
	return false
}

// IsImageOnly returns true if there are no video formats but there is a thumbnail
func (r *RichProbeInfo) IsImageOnly() bool {
	return !r.HasVideoFormats() && r.Thumbnail != ""
}

// BestAudioURL tries to find an audio-only or best-audio format URL.
// Returns empty string if none found.
func (r *RichProbeInfo) BestAudioURL() string {
	// look for audio-only format
	for _, f := range r.Formats {
		if (f.Acodec != "" && f.Acodec != "none") && (f.Vcodec == "" || f.Vcodec == "none") {
			return "" // no URL in ProbeFormat (yt-dlp needs separate call)
		}
	}
	return ""
}

// IsVideoFormatNotFoundError checks if the error string indicates yt-dlp found no video formats
func IsVideoFormatNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no video formats found") ||
		strings.Contains(msg, "requested format is not available") ||
		strings.Contains(msg, "no matching formats found")
}

func (y YTDLP) Probe(ctx context.Context, rawURL, format string, cookiesArgs []string) (ProbeInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	args := []string{
		"--no-playlist",
		"--format", format,
		"--no-warnings",
		"--dump-json",
		"--skip-download",
		"--retries", "3",
	}
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	out, errOut, err := run(ctx, y.Bin, args...)
	if err != nil {
		return ProbeInfo{}, fmt.Errorf("yt-dlp probe failed: %w: %s", err, sanitizeErrOutput(errOut))
	}
	var info ProbeInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return ProbeInfo{}, err
	}
	return info, nil
}

// ProbeRich runs yt-dlp with --dump-json (no format filter) to get full metadata including formats list
func (y YTDLP) ProbeRich(ctx context.Context, rawURL string, cookiesArgs []string) (RichProbeInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	args := []string{
		"--no-playlist",
		"--no-warnings",
		"--dump-json",
		"--skip-download",
		"--retries", "2",
	}
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	out, errOut, err := run(ctx, y.Bin, args...)
	if err != nil {
		return RichProbeInfo{}, fmt.Errorf("yt-dlp probe-rich failed: %w: %s", err, sanitizeErrOutput(errOut))
	}
	var info RichProbeInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return RichProbeInfo{}, err
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
		"--retries", "3",
		"--fragment-retries", "3",
		"-N", "16",
		"--concurrent-fragments", "16",
		"-o", template,
	}
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	_, errOut, err := run(ctx, y.Bin, args...)
	if err != nil {
		return "", fmt.Errorf("yt-dlp download failed: %w: %s", err, sanitizeErrOutput(errOut))
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

// DownloadThumbnail downloads the thumbnail/image of a post to outputDir/baseName.{ext}
func (y YTDLP) DownloadThumbnail(ctx context.Context, rawURL, outputDir, baseName string, cookiesArgs []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	args := []string{
		"--no-playlist",
		"--no-warnings",
		"--write-thumbnail",
		"--convert-thumbnails", "jpg",
		"--skip-download",
		"--retries", "2",
		"-o", filepath.Join(outputDir, baseName+".%(ext)s"),
	}
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	_, errOut, err := run(ctx, y.Bin, args...)
	if err != nil {
		return "", fmt.Errorf("yt-dlp thumbnail failed: %w: %s", err, sanitizeErrOutput(errOut))
	}
	// yt-dlp writes thumbnail as <baseName>.jpg or <baseName>.webp
	for _, ext := range []string{".jpg", ".jpeg", ".webp", ".png"} {
		p := filepath.Join(outputDir, baseName+ext)
		matches, _ := filepath.Glob(filepath.Join(outputDir, baseName+"*"+ext))
		if len(matches) > 0 {
			return matches[0], nil
		}
		_ = p
	}
	// fallback glob
	matches, _ := filepath.Glob(filepath.Join(outputDir, baseName+".*"))
	if len(matches) > 0 {
		return matches[0], nil
	}
	return "", fmt.Errorf("yt-dlp thumbnail not found in %s", outputDir)
}

// DownloadAudio downloads only the audio stream of the given URL
func (y YTDLP) DownloadAudio(ctx context.Context, rawURL, outputDir, baseName string, cookiesArgs []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	args := []string{
		"--no-playlist",
		"--no-warnings",
		"--format", "bestaudio[ext=m4a]/bestaudio",
		"--retries", "2",
		"-o", filepath.Join(outputDir, baseName+".%(ext)s"),
	}
	args = append(args, cookiesArgs...)
	args = append(args, rawURL)
	_, errOut, err := run(ctx, y.Bin, args...)
	if err != nil {
		return "", fmt.Errorf("yt-dlp audio-only failed: %w: %s", err, sanitizeErrOutput(errOut))
	}
	matches, _ := filepath.Glob(filepath.Join(outputDir, baseName+".*"))
	if len(matches) > 0 {
		return matches[0], nil
	}
	return "", fmt.Errorf("yt-dlp audio output not found")
}

func run(ctx context.Context, bin string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// sanitizeErrOutput truncates long stderr and removes paths/tokens for clean logging
func sanitizeErrOutput(errOut []byte) string {
	s := strings.TrimSpace(string(errOut))
	// take only last 500 chars
	if len(s) > 500 {
		s = "..." + s[len(s)-500:]
	}
	return s
}
