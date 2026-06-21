package users

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID             int64
	TelegramID     int64
	Username       string
	FullName       string
	Status         string
	DownloadsCount int64
}

type Profile struct {
	User
	SuccessDownloads int64
	FailedDownloads  int64
	SavedCount       int64
	TodayDownloads   int64
	LastDownloadAt   *time.Time
}

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

func (s *Service) Upsert(ctx context.Context, telegramID int64, username, fullName string) (User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		INSERT INTO users (telegram_id, username, full_name, last_seen_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (telegram_id) DO UPDATE
		SET username=EXCLUDED.username, full_name=EXCLUDED.full_name, last_seen_at=now(), updated_at=now()
		RETURNING id, telegram_id, COALESCE(username,''), COALESCE(full_name,''), status, downloads_count
	`, telegramID, username, fullName).Scan(&u.ID, &u.TelegramID, &u.Username, &u.FullName, &u.Status, &u.DownloadsCount)
	return u, err
}

func (s *Service) GetByTelegramID(ctx context.Context, telegramID int64) (User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		SELECT id, telegram_id, COALESCE(username,''), COALESCE(full_name,''), status, downloads_count
		FROM users WHERE telegram_id=$1
	`, telegramID).Scan(&u.ID, &u.TelegramID, &u.Username, &u.FullName, &u.Status, &u.DownloadsCount)
	return u, err
}

func (s *Service) IsBlocked(ctx context.Context, telegramID int64) (bool, error) {
	u, err := s.GetByTelegramID(ctx, telegramID)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	return u.Status == "BLOCKED", err
}

func (s *Service) IncrementDownloads(ctx context.Context, userID int64) error {
	_, err := s.db.Exec(ctx, `UPDATE users SET downloads_count=downloads_count+1, updated_at=now() WHERE id=$1`, userID)
	return err
}

func (s *Service) Profile(ctx context.Context, telegramID int64) (Profile, error) {
	u, err := s.GetByTelegramID(ctx, telegramID)
	if err != nil {
		return Profile{}, err
	}
	p := Profile{User: u}
	var last sql.NullTime
	err = s.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status='SUCCESS'),
			COUNT(*) FILTER (WHERE status='FAILED'),
			COUNT(*) FILTER (WHERE created_at::date=CURRENT_DATE),
			MAX(created_at)
		FROM downloads WHERE user_id=$1
	`, u.ID).Scan(&p.SuccessDownloads, &p.FailedDownloads, &p.TodayDownloads, &last)
	if err != nil {
		return Profile{}, err
	}
	if last.Valid {
		p.LastDownloadAt = &last.Time
	}
	_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM saved_media WHERE user_id=$1`, u.ID).Scan(&p.SavedCount)
	return p, nil
}

func (s *Service) DailyLimitReached(ctx context.Context, userID int64, limit int) (bool, error) {
	var count int
	err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM downloads WHERE user_id=$1 AND created_at::date=CURRENT_DATE`, userID).Scan(&count)
	return count >= limit, err
}

func (s *Service) GetByUsername(ctx context.Context, username string) (User, error) {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	var u User
	err := s.db.QueryRow(ctx, `
		SELECT id, telegram_id, COALESCE(username,''), COALESCE(full_name,''), status, downloads_count
		FROM users WHERE LOWER(username)=LOWER($1)
	`, username).Scan(&u.ID, &u.TelegramID, &u.Username, &u.FullName, &u.Status, &u.DownloadsCount)
	return u, err
}

func (s *Service) GetByID(ctx context.Context, id int64) (User, error) {
	var u User
	err := s.db.QueryRow(ctx, `
		SELECT id, telegram_id, COALESCE(username,''), COALESCE(full_name,''), status, downloads_count
		FROM users WHERE id=$1
	`, id).Scan(&u.ID, &u.TelegramID, &u.Username, &u.FullName, &u.Status, &u.DownloadsCount)
	return u, err
}

func (s *Service) ProfileByUserID(ctx context.Context, id int64) (Profile, error) {
	u, err := s.GetByID(ctx, id)
	if err != nil {
		return Profile{}, err
	}
	p := Profile{User: u}
	var last sql.NullTime
	err = s.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status='SUCCESS'),
			COUNT(*) FILTER (WHERE status='FAILED'),
			COUNT(*) FILTER (WHERE created_at::date=CURRENT_DATE),
			MAX(created_at)
		FROM downloads WHERE user_id=$1
	`, u.ID).Scan(&p.SuccessDownloads, &p.FailedDownloads, &p.TodayDownloads, &last)
	if err != nil {
		return Profile{}, err
	}
	if last.Valid {
		p.LastDownloadAt = &last.Time
	}
	_ = s.db.QueryRow(ctx, `SELECT COUNT(*) FROM saved_media WHERE user_id=$1`, u.ID).Scan(&p.SavedCount)
	return p, nil
}

func (s *Service) List(ctx context.Context, page, limit int) ([]User, int, error) {
	offset := (page - 1) * limit
	var total int
	err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.db.Query(ctx, `
		SELECT id, telegram_id, COALESCE(username,''), COALESCE(full_name,''), status, downloads_count
		FROM users ORDER BY id ASC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.TelegramID, &u.Username, &u.FullName, &u.Status, &u.DownloadsCount); err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

func (s *Service) Search(ctx context.Context, query string, limit int) ([]User, error) {
	query = strings.TrimSpace(query)
	queryPattern := "%" + query + "%"
	rows, err := s.db.Query(ctx, `
		SELECT id, telegram_id, COALESCE(username,''), COALESCE(full_name,''), status, downloads_count
		FROM users
		WHERE LOWER(username) ILIKE LOWER($1)
		   OR LOWER(full_name) ILIKE LOWER($1)
		   OR telegram_id::text ILIKE $1
		   OR id::text = $2
		ORDER BY id ASC LIMIT $3
	`, queryPattern, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.TelegramID, &u.Username, &u.FullName, &u.Status, &u.DownloadsCount); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Service) SetStatus(ctx context.Context, userID int64, status string) error {
	_, err := s.db.Exec(ctx, `UPDATE users SET status=$1, updated_at=now() WHERE id=$2`, status, userID)
	return err
}
