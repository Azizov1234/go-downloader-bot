package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"instagram-downloader-bot/internal/media"
)

type Service struct {
	TempDownloadDir string
}

func NewService(tempDownloadDir string) Service {
	return Service{TempDownloadDir: filepath.Clean(tempDownloadDir)}
}

func (s Service) DownloadBase(userTelegramID, downloadID int64, quality media.Quality) (dir string, base string, err error) {
	now := time.Now()
	dir = filepath.Join(
		s.TempDownloadDir,
		now.Format("2006"),
		now.Format("01"),
		now.Format("02"),
		fmt.Sprintf("user-%d", userTelegramID),
	)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	base = fmt.Sprintf("download-%d-%s", downloadID, strings.ToLower(string(quality)))
	return dir, base, nil
}

func (s Service) AudioPath(userTelegramID, downloadID int64) (string, error) {
	now := time.Now()
	dir := filepath.Join(
		s.TempDownloadDir,
		now.Format("2006"),
		now.Format("01"),
		now.Format("02"),
		fmt.Sprintf("user-%d", userTelegramID),
	)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("audio-%d.mp3", downloadID)), nil
}

func (s Service) RemoveSafe(path string) error {
	if path == "" {
		return nil
	}
	root, err := filepath.Abs(s.TempDownloadDir)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return err
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("unsafe remove outside temp dir: %s", path)
	}
	return os.Remove(target)
}
