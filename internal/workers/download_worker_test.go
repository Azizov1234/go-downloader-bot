package workers

import (
	"testing"

	"instagram-downloader-bot/internal/downloader"
	"instagram-downloader-bot/internal/media"
)

func TestSelectFormat_Auto_ProgressiveMP4Preferred(t *testing.T) {
	formats := []downloader.ProbeFormat{
		{FormatID: "dash_video", Ext: "mp4", Vcodec: "h264", Acodec: "none", Height: 1080, Width: 1920, Filesize: 50000000},
		{FormatID: "dash_audio", Ext: "m4a", Vcodec: "none", Acodec: "aac", Height: 0, Width: 0, Filesize: 5000000},
		{FormatID: "progressive_mp4_low", Ext: "mp4", Vcodec: "h264", Acodec: "aac", Height: 480, Width: 854, Filesize: 10000000},
		{FormatID: "progressive_mp4_high", Ext: "mp4", Vcodec: "h264", Acodec: "aac", Height: 720, Width: 1280, Filesize: 25000000},
		{FormatID: "progressive_webm", Ext: "webm", Vcodec: "vp9", Acodec: "opus", Height: 720, Width: 1280, Filesize: 20000000},
	}

	// For AUTO quality, we should select the best progressive MP4 stream (progressive_mp4_high, 720p)
	selected := selectFormat(formats, media.QualityAuto, "default_format")
	if selected.FormatID != "progressive_mp4_high" {
		t.Errorf("expected 'progressive_mp4_high', got '%s'", selected.FormatID)
	}
	if !selected.IsProgressive {
		t.Error("expected format to be progressive")
	}
	if selected.NeedsMerge {
		t.Error("expected format to not need merge")
	}
	if selected.Resolution != "1280x720" {
		t.Errorf("expected resolution '1280x720', got '%s'", selected.Resolution)
	}
}

func TestSelectFormat_Auto_FallbackToDASH(t *testing.T) {
	formats := []downloader.ProbeFormat{
		{FormatID: "dash_video_high", Ext: "mp4", Vcodec: "h264", Acodec: "none", Height: 1080, Width: 1920, Filesize: 50000000},
		{FormatID: "dash_video_low", Ext: "mp4", Vcodec: "h264", Acodec: "none", Height: 720, Width: 1280, Filesize: 25000000},
		{FormatID: "dash_audio_high", Ext: "m4a", Vcodec: "none", Acodec: "aac", Height: 0, Width: 0, Filesize: 5000000},
		{FormatID: "dash_audio_low", Ext: "m4a", Vcodec: "none", Acodec: "aac", Height: 0, Width: 0, Filesize: 2000000},
	}

	// For AUTO, since there is no progressive MP4, we select best video + best audio DASH
	selected := selectFormat(formats, media.QualityAuto, "default_format")
	if selected.FormatID != "dash_video_high+dash_audio_high" {
		t.Errorf("expected 'dash_video_high+dash_audio_high', got '%s'", selected.FormatID)
	}
	if selected.IsProgressive {
		t.Error("expected format not to be progressive")
	}
	if !selected.NeedsMerge {
		t.Error("expected format to need merge")
	}
	if selected.Resolution != "1920x1080" {
		t.Errorf("expected resolution '1920x1080', got '%s'", selected.Resolution)
	}
}

func TestSelectFormat_Original_HighestQuality(t *testing.T) {
	formats := []downloader.ProbeFormat{
		{FormatID: "dash_video_1080p", Ext: "mp4", Vcodec: "h264", Acodec: "none", Height: 1080, Width: 1920, Filesize: 50000000},
		{FormatID: "dash_audio_high", Ext: "m4a", Vcodec: "none", Acodec: "aac", Height: 0, Width: 0, Filesize: 5000000},
		{FormatID: "progressive_mp4_720p", Ext: "mp4", Vcodec: "h264", Acodec: "aac", Height: 720, Width: 1280, Filesize: 25000000},
	}

	// ORIGINAL quality should select the absolute best video and audio stream (dash_video_1080p + dash_audio_high)
	selected := selectFormat(formats, media.QualityOriginal, "default_format")
	if selected.FormatID != "dash_video_1080p+dash_audio_high" {
		t.Errorf("expected 'dash_video_1080p+dash_audio_high', got '%s'", selected.FormatID)
	}
	if selected.IsProgressive {
		t.Error("expected format not to be progressive")
	}
	if !selected.NeedsMerge {
		t.Error("expected format to need merge")
	}
}

func TestSelectFormat_ResolutionCapped(t *testing.T) {
	formats := []downloader.ProbeFormat{
		{FormatID: "dash_video_1080p", Ext: "mp4", Vcodec: "h264", Acodec: "none", Height: 1080, Width: 1920, Filesize: 50000000},
		{FormatID: "dash_video_720p", Ext: "mp4", Vcodec: "h264", Acodec: "none", Height: 720, Width: 1280, Filesize: 25000000},
		{FormatID: "dash_video_480p", Ext: "mp4", Vcodec: "h264", Acodec: "none", Height: 480, Width: 854, Filesize: 12000000},
		{FormatID: "dash_audio_high", Ext: "m4a", Vcodec: "none", Acodec: "aac", Height: 0, Width: 0, Filesize: 5000000},
	}

	// 720p cap should select dash_video_720p + dash_audio_high
	selected := selectFormat(formats, media.QualityP720, "default_format")
	if selected.FormatID != "dash_video_720p+dash_audio_high" {
		t.Errorf("expected 'dash_video_720p+dash_audio_high', got '%s'", selected.FormatID)
	}
	if selected.Resolution != "1280x720" {
		t.Errorf("expected resolution '1280x720', got '%s'", selected.Resolution)
	}
}
