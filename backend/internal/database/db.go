package database

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 25
	cfg.MinConns = 2
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.MaxConnLifetime = 1 * time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}
	slog.Info("database connected", "max_conns", cfg.MaxConns)
	return pool, nil
}

func RunMigrations(databaseURL string, migrationsPath string) error {
	m, err := migrate.New("file://"+migrationsPath, databaseURL)
	if err != nil {
		return fmt.Errorf("migration init failed: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration up failed: %w", err)
	}
	slog.Info("migrations applied")
	return nil
}

func RunSeed(pool *pgxpool.Pool, seedPath string) error {
	data, err := os.ReadFile(seedPath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("no seed file found, skipping")
			return nil
		}
		return fmt.Errorf("read seed file: %w", err)
	}
	_, err = pool.Exec(context.Background(), string(data))
	if err != nil {
		return fmt.Errorf("execute seed: %w", err)
	}
	slog.Info("seed data applied")
	return nil
}
