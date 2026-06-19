package users

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Admin struct {
	ID         int64
	TelegramID int64
	Role       string
}

type AdminService struct {
	db *pgxpool.Pool
}

func NewAdminService(db *pgxpool.Pool) *AdminService {
	return &AdminService{db: db}
}

func (s *AdminService) EnsureSuperAdmin(ctx context.Context, telegramID int64) error {
	if telegramID == 0 {
		return nil
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO admins (telegram_id, role)
		VALUES ($1, 'SUPERADMIN')
		ON CONFLICT (telegram_id) DO UPDATE SET role='SUPERADMIN', updated_at=now()
	`, telegramID)
	return err
}

func (s *AdminService) GetByTelegramID(ctx context.Context, telegramID int64) (Admin, error) {
	var a Admin
	err := s.db.QueryRow(ctx, `
		SELECT id, telegram_id, role FROM admins WHERE telegram_id=$1
	`, telegramID).Scan(&a.ID, &a.TelegramID, &a.Role)
	return a, err
}

func (s *AdminService) IsAdmin(ctx context.Context, telegramID int64) (bool, Admin, error) {
	a, err := s.GetByTelegramID(ctx, telegramID)
	if err == pgx.ErrNoRows {
		return false, Admin{}, nil
	}
	return err == nil, a, err
}

func (s *AdminService) List(ctx context.Context) ([]Admin, error) {
	rows, err := s.db.Query(ctx, `SELECT id, telegram_id, role FROM admins ORDER BY role DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Admin
	for rows.Next() {
		var a Admin
		if err := rows.Scan(&a.ID, &a.TelegramID, &a.Role); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *AdminService) Add(ctx context.Context, telegramID int64, role string) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO admins (telegram_id, role) VALUES ($1, $2)
		ON CONFLICT (telegram_id) DO UPDATE SET role=EXCLUDED.role, updated_at=now()
	`, telegramID, role)
	return err
}

func (s *AdminService) Remove(ctx context.Context, telegramID int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM admins WHERE telegram_id=$1 AND role <> 'SUPERADMIN'`, telegramID)
	return err
}
