package logs

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminActionLogService struct {
	db *pgxpool.Pool
}

func NewAdminActionLogService(db *pgxpool.Pool) *AdminActionLogService {
	return &AdminActionLogService{db: db}
}

func (s *AdminActionLogService) Write(ctx context.Context, adminID int64, action, details string) {
	_, _ = s.db.Exec(ctx, `
		INSERT INTO admin_action_logs (admin_id, action, details)
		VALUES ((SELECT id FROM admins WHERE telegram_id=$1), $2, $3)
	`, adminID, action, details)
}
