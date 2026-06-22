package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"
	"instagram-downloader-bot/internal/config"
	"instagram-downloader-bot/internal/downloader"
)

func main() {
	_ = godotenv.Load("../.env") // Load env from root
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	ytdlp := downloader.YTDLP{Bin: cfg.YTDLPBin}

	url := "https://www.instagram.com/reel/C8q8_NfMv-d/" 
	cookies := []string{"--cookies", cfg.InstagramCookiesFile}

	fmt.Println("--- STARTING BENCHMARK ---")
	fmt.Printf("Using cookies file: %s\n", cfg.InstagramCookiesFile)
	
	// 1. Probe Rich
	start := time.Now()
	richInfo, err := ytdlp.ProbeRich(ctx, url, cookies)
	fmt.Printf("ProbeRich took: %v, err: %v\n", time.Since(start), err)
	if err == nil {
		fmt.Printf("Title: %s\nIsImageOnly: %t\nFormats count: %d\n", richInfo.Title, richInfo.IsImageOnly(), len(richInfo.Formats))
	}

	// 2. Download (using format AUTO style)
	format := "best[ext=mp4]/best"
	start = time.Now()
	localPath, err := ytdlp.Download(ctx, url, format, ".", "test_out", cookies)
	fmt.Printf("Download (AUTO format) took: %v, err: %v, path: %s\n", time.Since(start), err, localPath)
}
