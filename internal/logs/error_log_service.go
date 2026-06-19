package logs

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ErrorLogService struct {
	db *pgxpool.Pool
}

func NewErrorLogService(db *pgxpool.Pool) *ErrorLogService {
	return &ErrorLogService{db: db}
}

func (s *ErrorLogService) Write(ctx context.Context, userID *int64, platform, action, message string, technical error) {
	var technicalMessage *string
	if technical != nil {
		v := technical.Error()
		technicalMessage = &v
	}
	_, _ = s.db.Exec(ctx, `
		INSERT INTO error_logs (user_id, platform, action, message, technical_error)
		VALUES ($1, $2, $3, $4, $5)
	`, userID, platform, action, message, technicalMessage)
}

func (s *ErrorLogService) Recent(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.db.Query(ctx, `
		SELECT action || ': ' || message || COALESCE(' [' || technical_error || ']', '')
		FROM error_logs ORDER BY created_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, rows.Err()
}
