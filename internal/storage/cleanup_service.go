package storage

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CleanupService struct {
	Root     string
	TTL      time.Duration
	Interval time.Duration
	Logger   *slog.Logger
}

func NewCleanupService(root string, ttl, interval time.Duration, logger *slog.Logger) CleanupService {
	return CleanupService{Root: filepath.Clean(root), TTL: ttl, Interval: interval, Logger: logger}
}

func (s CleanupService) RunOnce() {
	root, err := filepath.Abs(s.Root)
	if err != nil {
		s.Logger.Warn("cleanup root error", "error", err)
		return
	}
	cutoff := time.Now().Add(-s.TTL)
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		target, err := filepath.Abs(path)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, target)
		if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(target); err != nil {
				s.Logger.Warn("temp cleanup failed", "path", target, "error", err)
			}
		}
		return nil
	})
}

func (s CleanupService) Start(ctx context.Context) {
	s.RunOnce()
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.RunOnce()
		}
	}
}
