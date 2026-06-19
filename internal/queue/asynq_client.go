package queue

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"instagram-downloader-bot/internal/config"
)

type Client struct {
	client *asynq.Client
	cfg    config.Config
}

func RedisOpt(redisURL string) (asynq.RedisClientOpt, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return asynq.RedisClientOpt{}, err
	}
	return asynq.RedisClientOpt{Addr: opts.Addr, Username: opts.Username, Password: opts.Password, DB: opts.DB}, nil
}

func NewClient(cfg config.Config) (*Client, error) {
	opt, err := RedisOpt(cfg.RedisURL)
	if err != nil {
		return nil, err
	}
	return &Client{client: asynq.NewClient(opt), cfg: cfg}, nil
}

func NewInspector(cfg config.Config) (*asynq.Inspector, error) {
	opt, err := RedisOpt(cfg.RedisURL)
	if err != nil {
		return nil, err
	}
	return asynq.NewInspector(opt), nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) EnqueueDownload(ctx context.Context, payload DownloadTask) error {
	if payload.QueuedAt.IsZero() {
		payload.QueuedAt = time.Now()
	}
	return c.enqueue(ctx, TypeDownload, payload, QueueDownload)
}

func (c *Client) EnqueueAudioConvert(ctx context.Context, payload AudioConvertTask) error {
	return c.enqueue(ctx, TypeAudioConvert, payload, QueueAudioConvert)
}

func (c *Client) EnqueueSend(ctx context.Context, payload SendTask) error {
	return c.enqueue(ctx, TypeSend, payload, QueueSend)
}

func (c *Client) EnqueueNotification(ctx context.Context, payload NotificationTask) error {
	return c.enqueue(ctx, TypeNotification, payload, QueueNotification)
}

func (c *Client) enqueue(ctx context.Context, taskType string, payload any, queueName string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	task := asynq.NewTask(taskType, body, asynq.MaxRetry(c.cfg.JobAttempts), asynq.Queue(queueName), asynq.Timeout(20*time.Minute))
	_, err = c.client.EnqueueContext(ctx, task)
	return err
}
