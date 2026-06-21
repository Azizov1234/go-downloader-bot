-- +goose Up
UPDATE bot_settings
SET telegram_api_mode = 'local',
    telegram_local_max_upload_mb = 2000,
    telegram_max_upload_mb = 2000,
    max_video_file_size_mb = 2000,
    max_audio_file_size_mb = 2000,
    telegram_use_local_file_path = true,
    require_local_bot_api_for_large_files = true
WHERE id = 1;

-- +goose Down
