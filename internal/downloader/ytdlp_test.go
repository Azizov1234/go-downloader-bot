package downloader

import (
	"errors"
	"testing"
)

func TestIsVideoFormatNotFoundError(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		expect bool
	}{
		{name: "nil", err: nil, expect: false},
		{name: "no video formats found", err: errors.New("yt-dlp probe failed: exit status 1: ERROR: No video formats found"), expect: true},
		{name: "requested format not available", err: errors.New("yt-dlp download failed: requested format is not available"), expect: true},
		{name: "no matching formats", err: errors.New("no matching formats found; please specify different --video-format"), expect: true},
		{name: "unrelated error", err: errors.New("network timeout"), expect: false},
		{name: "private post", err: errors.New("login required"), expect: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := IsVideoFormatNotFoundError(c.err)
			if got != c.expect {
				t.Errorf("IsVideoFormatNotFoundError(%v) = %v, want %v", c.err, got, c.expect)
			}
		})
	}
}

func TestRichProbeInfo_HasVideoFormats(t *testing.T) {
	// Image-only post: thumbnail but no video formats
	imagePost := RichProbeInfo{
		ProbeInfo: ProbeInfo{Thumbnail: "https://cdn.instagram.com/thumb.jpg", Title: "My post"},
		Formats: []ProbeFormat{
			{FormatID: "audio_only", Ext: "m4a", Vcodec: "none", Acodec: "mp4a.40.2"},
		},
	}
	if imagePost.HasVideoFormats() {
		t.Error("expected image-only post to have no video formats")
	}
	if !imagePost.IsImageOnly() {
		t.Error("expected image-only post to be detected as image-only")
	}

	// Video post
	videoPost := RichProbeInfo{
		ProbeInfo: ProbeInfo{Thumbnail: "https://cdn.instagram.com/thumb.jpg", Title: "My video"},
		Formats: []ProbeFormat{
			{FormatID: "hd", Ext: "mp4", Vcodec: "h264", Acodec: "mp4a.40.2", Height: 720},
		},
	}
	if !videoPost.HasVideoFormats() {
		t.Error("expected video post to have video formats")
	}
	if videoPost.IsImageOnly() {
		t.Error("expected video post NOT to be image-only")
	}

	// Empty (no formats at all, no thumbnail)
	emptyPost := RichProbeInfo{}
	if emptyPost.HasVideoFormats() {
		t.Error("expected empty post to have no video formats")
	}
	if emptyPost.IsImageOnly() {
		t.Error("expected post with no thumbnail to NOT be image-only (can't send image)")
	}
}

func TestRichProbeInfo_CarouselVideoFormats(t *testing.T) {
	// Carousel with videos in entries
	carousel := RichProbeInfo{
		ProbeInfo: ProbeInfo{Title: "Carousel", Thumbnail: "https://example.com/t.jpg"},
		Formats:   []ProbeFormat{},
		Entries: []ProbeEntry{
			{
				Formats: []ProbeFormat{
					{FormatID: "hd", Vcodec: "h264", Acodec: "aac", Height: 1080},
				},
			},
		},
	}
	if !carousel.HasVideoFormats() {
		t.Error("expected carousel with video entries to have video formats")
	}
}

func TestSanitizeErrOutput(t *testing.T) {
	long := make([]byte, 600)
	for i := range long {
		long[i] = 'x'
	}
	result := sanitizeErrOutput(long)
	if len(result) > 510 { // 500 + "..." prefix
		t.Errorf("expected truncated output, got len=%d", len(result))
	}
	if result[:3] != "..." {
		t.Errorf("expected truncated output to start with '...', got %q", result[:3])
	}
}
