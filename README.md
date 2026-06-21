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
- Telegram API mode: cloud/local
- cloud/local upload limit
- Local Bot API health check
- donat karta va matn
- welcome/help text

## 2GB gacha video yuborish

Cloud Telegram Bot API odatda 50 MB atrofida upload limitga uriladi. 50 MB dan katta, masalan 148 MB, 500 MB, 1 GB yoki 2 GB gacha videolarni yuborish uchun Local Telegram Bot API Server kerak.

Talablar:

- my.telegram.org orqali `api_id` va `api_hash` oling.
- `telegram-bot-api` serverga o'rnatilgan bo'lsin.
- Bot va `telegram-bot-api` bir xil VPS/server ichida ishlasin.
- `telegram-bot-api` `--local` rejimida ishlasin.
- `uploads` papkasini local Bot API process o'qiy olsin.

Local Bot API serverni ishga tushirish namunasi:

```bash
telegram-bot-api \
  --api-id=YOUR_API_ID \
  --api-hash=YOUR_API_HASH \
  --local \
  --http-port=8081
```

Bot `.env` local rejim:

```env
TELEGRAM_API_MODE=local
TELEGRAM_API_ID=YOUR_API_ID
TELEGRAM_API_HASH=YOUR_API_HASH
TELEGRAM_LOCAL_API_URL=http://127.0.0.1:8081
TELEGRAM_LOCAL_MAX_UPLOAD_MB=2000
TELEGRAM_USE_LOCAL_FILE_PATH=true
REQUIRE_LOCAL_BOT_API_FOR_LARGE_FILES=true
```

Windowsda `.env`dagi `TELEGRAM_API_ID` va `TELEGRAM_API_HASH` to'ldirilgandan keyin local Bot API serverni script bilan start qilish mumkin:

```powershell
.\start-telegram-api.ps1
```

Cloud rejim:

```env
TELEGRAM_API_MODE=cloud
TELEGRAM_CLOUD_API_URL=https://api.telegram.org
TELEGRAM_CLOUD_MAX_UPLOAD_MB=50
```

Bot cache topilsa `telegram_file_id` orqali darhol yuboradi: download qilmaydi, local fayl qidirmaydi. Agar eski `file_id` ishlamasa, u tozalanadi va video qayta download qilinadi.

Local rejimda video/audio serverdagi local file path orqali yuboriladi. Yuborish tugagandan keyin temp fayl o'chiriladi. Fayllar quyidagi ko'rinishda saqlanadi:

```text
uploads/temp/downloads/YYYY/MM/DD/user-{telegramId}/download-{downloadId}-{quality}.mp4
```

## systemd

`systemd/telegram-bot-api.service`, `systemd/instagram-bot.service` va `systemd/instagram-worker.service` namunalarini server yo'llariga moslab o'zgartiring.

```bash
sudo cp systemd/telegram-bot-api.service /etc/systemd/system/
sudo cp systemd/instagram-*.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now telegram-bot-api
sudo systemctl enable --now instagram-bot instagram-worker
```

`telegram-bot-api.service` namunasi:

```ini
[Unit]
Description=Local Telegram Bot API Server
After=network.target

[Service]
ExecStart=/usr/local/bin/telegram-bot-api --api-id=YOUR_API_ID --api-hash=YOUR_API_HASH --local --http-port=8081
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target

## So'nggi O'zgarishlar va Yangilanishlar (June 2026)

Telegram Local Bot API Server orqali 2GB gacha fayllarni yuklash logikasi, xatolik xabarlari, Downloader fallback tizimi va testlar quyidagi ko'rinishda to'g'rilandi:

### 1. Telegram Local API orqali Multipart Formatda Yuklash va Limitlar
- **Tegilgan kod**: `internal/media/delivery_service.go`, `internal/settings/settings_service.go`, `cmd/bot/main.go`, `cmd/worker/main.go`.
- **Amalga oshirildi**: 
  - Telegram local va cloud rejimlari uchun limit tekshiruvlari aniq va ishonchli qilindi (`TelegramLocalMaxUploadMB` / `TelegramCloudMaxUploadMB`).
  - Local API server local file path stringlarni qabul qilmagani uchun, local yuklash rejimi **multipart upload** formatiga o'tkazildi (xuddi `curl -F video=@/path/file.mp4` kabi).
  - Local rejimda 50MB dan katta bo'lgan fayllarni yuborishdan oldin local API serverining holati (`getMe` orqali) tekshiriladi. Agar u o'chiq bo'lsa, xatolik darhol qaytariladi (cloud rejimiga noto'g'ri o'tib ketmaydi).
  - Kichik hajmdagi (50MB gacha) fayllarni local API orqali yuklash o'xshamasagina avtomatik ravishda cloud API orqali yuklashga fallback qilinadi.
  - Har safar bot start bo'lganida database `EnsureDefaults` orqali database limit sozlamalarini `.env` dagi qiymatlar bilan sinxronizatsiya qiladi.
  - Yuklashdan oldin barcha ma'lumotlar (absolute path, exists status, file size, api mode, limit, url va method=sendMultipartLocalAPI) logga yoziladi. Telegram API xatolik bersa, HTTP status, response body, masked token, file path va size logda to'liq ko'rsatiladi.

### 2. Downloader Fallback (`gallery-dl`)
- **Tegilgan kod**: `internal/downloader/` (yangi files: `downloader.go`, `gallerydl.go`, `fallback.go`), `internal/workers/download_worker.go`, `cmd/worker/main.go`, `internal/config/config.go`.
- **Amalga oshirildi**:
  - `Downloader` interfeysi orqali `yt-dlp` va `gallery-dl` abstraktsiya qilindi.
  - Agar `yt-dlp` orqali yuklash xatolik bilan tugasa, avtomatik ravishda `gallery-dl` orqali yuklashga urinadi. Bu Instagram reels yoki postlar yuklash ishonchliligini oshiradi.
  - `.env` fayliga `GALLERYDL_BIN` (default `gallery-dl`) o'zgaruvchisi qo'shildi.

### 3. Xatolik Xabarlari, Faylni Saqlash va normalizer
- **Tegilgan kod**: `internal/telegram/messages.go`, `internal/workers/send_worker.go`, `internal/instagram/normalizer.go`.
- **Amalga oshirildi**:
  - `TelegramUploadTooLarge` funksiyasi mode-aware holatga keltirildi. Local rejimda 50MB error ko'rsatmaydi va o'rniga dynamic limit ko'rsatadi (`Local Bot API limiti: 2000 MB`).
  - Local API server ishlamay qolganda yoki ulanish uzilganda dinamik ravishda to'g'ri URL portini ko'rsatuvchi xabar chiqariladi: `Local Telegram Bot API ishlamayapti yoki 127.0.0.1:8081 ulanmayapti`.
  - Download xatosi va Send xatosi xabarlari ajratildi. Yuklab olgandan keyin Telegramga jo'natib bo'lmaganida: `Media yuklandi, lekin Telegramga yuborishda xatolik bo'ldi.` xabari qaytariladi.
  - `.env` faylida `KEEP_FAILED_DOWNLOADS=true` sozlangan bo'lsa, Telegramga yuborish o'xshamagan fayllar diskdan darhol o'chirilmaydi.
  - Instagram `/reels/{shortcode}/` havolalari ham qabul qilinib, avtomatik ravishda `/reel/{shortcode}/` ko'rinishida normalize qilinadi.

### 4. Unit Testlar
- **Yangi yozilgan testlar**: 
  - `internal/media/delivery_service_test.go` (Local mode allowed with multipart verification, Cloud mode rejected, Local mode oversized check).
  - `internal/telegram/messages_test.go` (Local/Cloud error message va Local Bot API unavailable dynamic format testlari).
  - `internal/instagram/normalizer_test.go` (Reels normalizer testi).
- **Tekshirish**: `go test ./...` komandasi orqali barcha testlar muvaffaqiyatli o'tadi.
```
