package db

import (
	"context"
	stddb "database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"instagram-downloader-bot/internal/config"
)

func NewPostgres(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func RunMigrations(databaseURL, dir string) error {
	conn, err := stddb.Open("pgx", databaseURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(conn, dir)
}
