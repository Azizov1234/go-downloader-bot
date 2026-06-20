-- +goose Up
ALTER TABLE bot_settings
    ADD COLUMN IF NOT EXISTS telegram_api_mode TEXT NOT NULL DEFAULT 'cloud',
    ADD COLUMN IF NOT EXISTS telegram_cloud_max_upload_mb BIGINT NOT NULL DEFAULT 50,
    ADD COLUMN IF NOT EXISTS telegram_local_max_upload_mb BIGINT NOT NULL DEFAULT 2000,
    ADD COLUMN IF NOT EXISTS telegram_use_local_file_path BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS require_local_bot_api_for_large_files BOOLEAN NOT NULL DEFAULT true;

UPDATE bot_settings
SET telegram_api_mode = 'cloud'
WHERE telegram_api_mode NOT IN ('cloud', 'local');

-- +goose Down
ALTER TABLE bot_settings
    DROP COLUMN IF EXISTS require_local_bot_api_for_large_files,
    DROP COLUMN IF EXISTS telegram_use_local_file_path,
    DROP COLUMN IF EXISTS telegram_local_max_upload_mb,
    DROP COLUMN IF EXISTS telegram_cloud_max_upload_mb,
    DROP COLUMN IF EXISTS telegram_api_mode;
