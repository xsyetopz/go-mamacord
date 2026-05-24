package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

type Options struct {
	DSN string
}

func Open(ctx context.Context, opts Options) (*sql.DB, error) {
	dsn := strings.TrimSpace(opts.DSN)
	if dsn == "" {
		return nil, errors.New("postgres dsn is required")
	}

	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}

	db := stdlib.OpenDB(*cfg)
	db.SetConnMaxLifetime(postgresConnMaxLifetime)
	db.SetMaxIdleConns(postgresMaxIdleConns)
	db.SetMaxOpenConns(postgresMaxOpenConns)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return db, nil
}

const (
	postgresConnMaxLifetime = 30 * time.Minute
	postgresMaxIdleConns    = 5
	postgresMaxOpenConns    = 20
)
