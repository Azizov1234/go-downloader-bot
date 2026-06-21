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
