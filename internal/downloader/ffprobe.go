package downloader

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"instagram-downloader-bot/internal/media"
)

type FFProbe struct {
	Bin string
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		AvgFrameRate string `json:"avg_frame_rate"`
		Duration     string `json:"duration"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		Size     string `json:"size"`
	} `json:"format"`
}

func (p FFProbe) Metadata(ctx context.Context, path string) (media.Metadata, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, p.Bin, "-v", "error", "-print_format", "json", "-show_format", "-show_streams", path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return media.Metadata{}, err
	}
	var parsed ffprobeOutput
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		return media.Metadata{}, err
	}
	stat, _ := os.Stat(path)
	md := media.Metadata{}
	if stat != nil {
		md.FileSize = stat.Size()
	}
	for _, stream := range parsed.Streams {
		if stream.CodecType != "video" {
			continue
		}
		if stream.Width > 0 {
			v := stream.Width
			md.Width = &v
		}
		if stream.Height > 0 {
			v := stream.Height
			md.Height = &v
		}
		if stream.CodecName != "" {
			v := stream.CodecName
			md.Codec = &v
		}
		if fps := parseFPS(stream.AvgFrameRate); fps > 0 {
			md.FPS = &fps
		}
		if d := parseDurationSeconds(stream.Duration); d > 0 {
			md.Duration = &d
		}
		break
	}
	if md.Duration == nil {
		if d := parseDurationSeconds(parsed.Format.Duration); d > 0 {
			md.Duration = &d
		}
	}
	if md.FileSize == 0 {
		if n, err := strconv.ParseInt(parsed.Format.Size, 10, 64); err == nil {
			md.FileSize = n
		}
	}
	return md, nil
}

func parseDurationSeconds(raw string) int {
	if raw == "" {
		return 0
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return int(f + 0.5)
}

func parseFPS(raw string) float64 {
	parts := strings.Split(raw, "/")
	if len(parts) != 2 {
		return 0
	}
	a, errA := strconv.ParseFloat(parts[0], 64)
	b, errB := strconv.ParseFloat(parts[1], 64)
	if errA != nil || errB != nil || b == 0 {
		return 0
	}
	return a / b
}
