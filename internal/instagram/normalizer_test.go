package instagram

import (
	"testing"
)

func TestNormalize_Reels(t *testing.T) {
	parsed, err := Normalize("https://www.instagram.com/reels/C8q8_NfMv-d/")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	expectedNormalized := "https://www.instagram.com/reel/C8q8_NfMv-d/"
	if parsed.NormalizedURL != expectedNormalized {
		t.Errorf("expected normalized url to be '%s', got '%s'", expectedNormalized, parsed.NormalizedURL)
	}
	if parsed.Kind != "reel" {
		t.Errorf("expected kind to be 'reel', got '%s'", parsed.Kind)
	}
	if parsed.Shortcode != "C8q8_NfMv-d" {
		t.Errorf("expected shortcode to be 'C8q8_NfMv-d', got '%s'", parsed.Shortcode)
	}
}

func TestNormalize_Post(t *testing.T) {
	parsed, err := Normalize("https://www.instagram.com/p/DZZj9NmoRXV/")
	if err != nil {
		t.Fatalf("expected no error for /p/ URL, got %v", err)
	}
	if parsed.Shortcode != "DZZj9NmoRXV" {
		t.Errorf("expected shortcode 'DZZj9NmoRXV', got '%s'", parsed.Shortcode)
	}
	expected := "https://www.instagram.com/p/DZZj9NmoRXV/"
	if parsed.NormalizedURL != expected {
		t.Errorf("expected normalized url '%s', got '%s'", expected, parsed.NormalizedURL)
	}
	if parsed.Kind != "p" {
		t.Errorf("expected kind 'p', got '%s'", parsed.Kind)
	}
}

func TestNormalize_ReelSingular(t *testing.T) {
	parsed, err := Normalize("https://www.instagram.com/reel/ABC123def/")
	if err != nil {
		t.Fatalf("expected no error for /reel/ URL, got %v", err)
	}
	if parsed.Kind != "reel" {
		t.Errorf("expected kind 'reel', got '%s'", parsed.Kind)
	}
	if parsed.Shortcode != "ABC123def" {
		t.Errorf("expected shortcode 'ABC123def', got '%s'", parsed.Shortcode)
	}
}

func TestNormalize_TV(t *testing.T) {
	parsed, err := Normalize("https://www.instagram.com/tv/BqxX0-FH3L_/")
	if err != nil {
		t.Fatalf("expected no error for /tv/ URL, got %v", err)
	}
	if parsed.Kind != "tv" {
		t.Errorf("expected kind 'tv', got '%s'", parsed.Kind)
	}
	if parsed.Shortcode != "BqxX0-FH3L_" {
		t.Errorf("expected shortcode 'BqxX0-FH3L_', got '%s'", parsed.Shortcode)
	}
}

func TestNormalize_QueryParamsStripped(t *testing.T) {
	// Query params should not appear in normalized URL
	parsed, err := Normalize("https://www.instagram.com/p/DZZj9NmoRXV/?utm_source=ig_web_copy_link&igsh=abc123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	expected := "https://www.instagram.com/p/DZZj9NmoRXV/"
	if parsed.NormalizedURL != expected {
		t.Errorf("expected normalized url '%s' (no query params), got '%s'", expected, parsed.NormalizedURL)
	}
	if parsed.Shortcode != "DZZj9NmoRXV" {
		t.Errorf("expected shortcode 'DZZj9NmoRXV', got '%s'", parsed.Shortcode)
	}
}

func TestNormalize_ReelsPluralToSingular(t *testing.T) {
	// /reels/ should normalize to /reel/
	parsed, err := Normalize("https://www.instagram.com/reels/C8q8_NfMv-d/?igsh=whatever")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	expected := "https://www.instagram.com/reel/C8q8_NfMv-d/"
	if parsed.NormalizedURL != expected {
		t.Errorf("expected '%s', got '%s'", expected, parsed.NormalizedURL)
	}
}

func TestNormalize_UnsupportedPath(t *testing.T) {
	_, err := Normalize("https://www.instagram.com/username/")
	if err == nil {
		t.Error("expected error for username-only URL, got nil")
	}
}
