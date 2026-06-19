package workers

import (
	"context"

	"github.com/hibiken/asynq"

	"instagram-downloader-bot/internal/storage"
)

type CleanupWorker struct {
	cleanup storage.CleanupService
}

func NewCleanupWorker(cleanup storage.CleanupService) *CleanupWorker {
	return &CleanupWorker{cleanup: cleanup}
}

func (w *CleanupWorker) ProcessTask(ctx context.Context, task *asynq.Task) error {
	w.cleanup.RunOnce()
	return nil
}
