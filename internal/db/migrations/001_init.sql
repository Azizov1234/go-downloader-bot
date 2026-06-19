-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT NOT NULL UNIQUE,
    username TEXT,
    full_name TEXT,
    status TEXT NOT NULL DEFAULT 'ACTIVE',
    downloads_count BIGINT NOT NULL DEFAULT 0,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS admins (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT NOT NULL UNIQUE,
    role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bot_settings (
    id BIGSERIAL PRIMARY KEY,
    bot_online BOOLEAN NOT NULL DEFAULT true,
    maintenance_mode BOOLEAN NOT NULL DEFAULT false,
    max_video_file_size_mb BIGINT NOT NULL DEFAULT 500,
    max_audio_file_size_mb BIGINT NOT NULL DEFAULT 100,
    telegram_max_upload_mb BIGINT NOT NULL DEFAULT 2000,
    welcome_text TEXT NOT NULL DEFAULT 'Instagram havolasini yuboring.',
    help_text TEXT NOT NULL DEFAULT 'Public yoki ruxsatli Instagram link yuboring.',
    donate_card_number TEXT NOT NULL DEFAULT '',
    donate_card_owner TEXT NOT NULL DEFAULT '',
    donate_text TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS media_files (
    id BIGSERIAL PRIMARY KEY,
    original_url TEXT NOT NULL,
    normalized_url TEXT NOT NULL UNIQUE,
    instagram_shortcode TEXT,
    platform TEXT NOT NULL DEFAULT 'instagram',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS media_variants (
    id BIGSERIAL PRIMARY KEY,
    media_file_id BIGINT NOT NULL REFERENCES media_files(id) ON DELETE CASCADE,
    normalized_url TEXT NOT NULL,
    original_url TEXT NOT NULL,
    instagram_shortcode TEXT,
    variant_type TEXT NOT NULL,
    quality TEXT NOT NULL,
    telegram_file_id TEXT,
    telegram_file_unique_id TEXT,
    width INTEGER,
    height INTEGER,
    duration INTEGER,
    fps DOUBLE PRECISION,
    codec TEXT,
    file_size BIGINT,
    status TEXT NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (normalized_url, variant_type, quality)
);

CREATE TABLE IF NOT EXISTS downloads (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    media_variant_id BIGINT REFERENCES media_variants(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'PENDING',
    cache_hit BOOLEAN NOT NULL DEFAULT false,
    cache_lookup_ms BIGINT NOT NULL DEFAULT 0,
    queue_wait_ms BIGINT NOT NULL DEFAULT 0,
    download_ms BIGINT NOT NULL DEFAULT 0,
    convert_ms BIGINT NOT NULL DEFAULT 0,
    send_ms BIGINT NOT NULL DEFAULT 0,
    total_ms BIGINT NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS saved_media (
    id BIGSERIAL PRIMARY KEY,
    save_number BIGINT NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    media_variant_id BIGINT NOT NULL REFERENCES media_variants(id) ON DELETE CASCADE,
    telegram_file_id TEXT NOT NULL,
    platform TEXT NOT NULL DEFAULT 'instagram',
    quality TEXT NOT NULL,
    variant_type TEXT NOT NULL,
    original_url TEXT NOT NULL,
    normalized_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, media_variant_id)
);

CREATE SEQUENCE IF NOT EXISTS saved_media_number_seq START WITH 1 INCREMENT BY 1;

CREATE TABLE IF NOT EXISTS error_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    platform TEXT,
    action TEXT NOT NULL,
    message TEXT NOT NULL,
    technical_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS admin_action_logs (
    id BIGSERIAL PRIMARY KEY,
    admin_id BIGINT REFERENCES admins(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    details TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS daily_stats (
    id BIGSERIAL PRIMARY KEY,
    date DATE NOT NULL UNIQUE,
    users_count BIGINT NOT NULL DEFAULT 0,
    downloads_count BIGINT NOT NULL DEFAULT 0,
    success_count BIGINT NOT NULL DEFAULT 0,
    failed_count BIGINT NOT NULL DEFAULT 0,
    cache_hit_count BIGINT NOT NULL DEFAULT 0,
    cache_miss_count BIGINT NOT NULL DEFAULT 0,
    audio_count BIGINT NOT NULL DEFAULT 0,
    video_count BIGINT NOT NULL DEFAULT 0,
    oversized_rejected_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS donate_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    amount NUMERIC(18,2),
    status TEXT NOT NULL DEFAULT 'PENDING',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id);
CREATE INDEX IF NOT EXISTS idx_media_files_normalized_url ON media_files(normalized_url);
CREATE INDEX IF NOT EXISTS idx_media_files_instagram_shortcode ON media_files(instagram_shortcode);
CREATE INDEX IF NOT EXISTS idx_media_variants_normalized_url ON media_variants(normalized_url);
CREATE INDEX IF NOT EXISTS idx_media_variants_telegram_file_id ON media_variants(telegram_file_id);
CREATE INDEX IF NOT EXISTS idx_media_variants_quality ON media_variants(quality);
CREATE INDEX IF NOT EXISTS idx_media_variants_variant_type ON media_variants(variant_type);
CREATE INDEX IF NOT EXISTS idx_downloads_user_id ON downloads(user_id);
CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status);
CREATE INDEX IF NOT EXISTS idx_downloads_created_at ON downloads(created_at);
CREATE INDEX IF NOT EXISTS idx_saved_media_user_id ON saved_media(user_id);
CREATE INDEX IF NOT EXISTS idx_saved_media_save_number ON saved_media(save_number);
CREATE INDEX IF NOT EXISTS idx_admins_telegram_id ON admins(telegram_id);

-- +goose Down
DROP TABLE IF EXISTS donate_logs;
DROP TABLE IF EXISTS daily_stats;
DROP TABLE IF EXISTS admin_action_logs;
DROP TABLE IF EXISTS error_logs;
DROP TABLE IF EXISTS saved_media;
DROP SEQUENCE IF EXISTS saved_media_number_seq;
DROP TABLE IF EXISTS downloads;
DROP TABLE IF EXISTS media_variants;
DROP TABLE IF EXISTS media_files;
DROP TABLE IF EXISTS bot_settings;
DROP TABLE IF EXISTS admins;
DROP TABLE IF EXISTS users;
