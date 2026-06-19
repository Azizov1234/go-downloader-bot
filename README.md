# Instagram Downloader Telegram Bot

Go asosidagi tezkor Instagram Downloader Telegram bot. Bot faqat public yoki ruxsatli Instagram havolalar bilan ishlaydi, eski yuklangan media esa `telegram_file_id` cache orqali darhol yuboriladi.

## Talablar

- Go 1.23+
- PostgreSQL
- Redis
- yt-dlp
- ffmpeg va ffprobe

## Go install

Linux:

```bash
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
source ~/.profile
go version
```

Windows uchun Go installer: https://go.dev/dl/

## PostgreSQL setup

```bash
sudo apt install postgresql postgresql-contrib
sudo -u postgres createdb instagram_downloader
sudo -u postgres psql -c "ALTER USER postgres WITH PASSWORD 'postgres';"
```

## Redis setup

```bash
sudo apt install redis-server
sudo systemctl enable --now redis-server
redis-cli ping
```

## yt-dlp install

```bash
python3 -m pip install -U yt-dlp
yt-dlp --version
```

## ffmpeg install

```bash
sudo apt install ffmpeg
ffmpeg -version
ffprobe -version
```

## O'rnatish

```bash
go mod tidy
cp .env.example .env
```

`.env` ichida `BOT_TOKEN`, `DATABASE_URL`, `REDIS_URL`, `SUPERADMIN_TELEGRAM_ID` va kerakli limitlarni to'ldiring.

PostgreSQL bazasini yarating:

```bash
createdb instagram_downloader
```

Migration:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir internal/db/migrations postgres "$DATABASE_URL" up
```

Build:

```bash
go build ./...
```

Bot:

```bash
go run ./cmd/bot
```

Worker:

```bash
go run ./cmd/worker
```

## Architecture

- `cmd/bot` Telegram update polling, user/admin handlerlar va cache-first flow.
- `cmd/worker` Asynq workerlar: download, audio convert, send, cleanup, notification.
- `internal/instagram` Instagram URL parser, normalizer, cookies va yt-dlp format builder.
- `internal/downloader` yt-dlp, ffprobe va ffmpeg wrapperlari.
- `internal/media` media, variant, cache lookup, Telegram delivery va download audit.
- `internal/queue` Asynq client, payloadlar va Redis in-flight lock.
- `internal/settings`, `internal/users`, `internal/saved`, `internal/stats`, `internal/logs` PostgreSQL service qatlamlari.

## Cache

Cache kaliti:

```text
instagram:{normalizedUrl}:{variantType}:{quality}
```

Bot link kelganda avval `media_variants.telegram_file_id` ni tekshiradi. Hit bo'lsa `yt-dlp`, `ffprobe`, `ffmpeg`, local file va queue ishlamaydi. Agar Telegram eski `file_id`ni rad etsa, variantdagi file id tozalanadi va download queuega qayta yuboriladi.

## Tanlov paneli

Instagram link miss bo'lsa bitta inline panel chiqadi:

- Video yoki Audio MP3
- Auto, Original, 1080p, 720p, 480p, kichik hajm
- Yuklash yoki bekor qilish

Default tanlov: `VIDEO + AUTO`. User sifatni bosmasa ham `Yuklash` video Auto/Asl holatda queuega tushadi.

## Admin

`/admin` faqat adminlar uchun. Birinchi startda `.env` dagi `SUPERADMIN_TELEGRAM_ID` `SUPERADMIN` sifatida seed qilinadi. Sozlamalar Postgres `bot_settings` jadvalidan o'qiladi:

- online/offline
- maintenance
- max video/audio MB
- donat karta va matn
- welcome/help text

## systemd

`systemd/instagram-bot.service` va `systemd/instagram-worker.service` namunalarini server yo'llariga moslab o'zgartiring.

```bash
sudo cp systemd/instagram-*.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now instagram-bot instagram-worker
```
