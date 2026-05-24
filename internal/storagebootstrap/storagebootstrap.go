package storagebootstrap

import (
	"context"
	"fmt"

	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	"github.com/xsyetopz/go-mamacord/internal/config"
	migrate "github.com/xsyetopz/go-mamacord/internal/migration"
	"github.com/xsyetopz/go-mamacord/internal/postgres"
	postgresstore "github.com/xsyetopz/go-mamacord/internal/storage/postgres"
)

type RuntimeStore interface {
	commandruntime.Store
	Close() error
}

func OpenRuntimeStore(ctx context.Context, cfg config.Config) (RuntimeStore, int, error) {
	switch cfg.StorageBackend {
	case "", config.StorageBackendPostgres:
		return openPostgresRuntimeStore(ctx, cfg)
	default:
		return nil, 0, fmt.Errorf("unsupported storage backend %q", cfg.StorageBackend)
	}
}

func MigrationStatus(ctx context.Context, cfg config.Config) (migrate.Status, error) {
	runner, err := migrate.New(migrate.Options{
		Dir: cfg.Migrations,
	})
	if err != nil {
		return migrate.Status{}, err
	}

	switch cfg.StorageBackend {
	case "", config.StorageBackendPostgres:
		db, err := postgres.Open(ctx, postgres.Options{DSN: cfg.PostgresDSN})
		if err != nil {
			return migrate.Status{}, err
		}
		defer db.Close()
		return runner.Status(ctx, db)
	default:
		return migrate.Status{}, fmt.Errorf("unsupported storage backend %q", cfg.StorageBackend)
	}
}

func MigrateUp(ctx context.Context, cfg config.Config) (migrate.Status, error) {
	runner, err := migrate.New(migrate.Options{
		Dir: cfg.Migrations,
	})
	if err != nil {
		return migrate.Status{}, err
	}

	switch cfg.StorageBackend {
	case "", config.StorageBackendPostgres:
		db, err := postgres.Open(ctx, postgres.Options{DSN: cfg.PostgresDSN})
		if err != nil {
			return migrate.Status{}, err
		}
		defer db.Close()
		return runner.Up(ctx, db)
	default:
		return migrate.Status{}, fmt.Errorf("unsupported storage backend %q", cfg.StorageBackend)
	}
}

func openPostgresRuntimeStore(ctx context.Context, cfg config.Config) (RuntimeStore, int, error) {
	db, err := postgres.Open(ctx, postgres.Options{DSN: cfg.PostgresDSN})
	if err != nil {
		return nil, 0, err
	}

	runner, err := migrate.New(migrate.Options{
		Dir: cfg.Migrations,
	})
	if err != nil {
		_ = db.Close()
		return nil, 0, err
	}
	status, err := runner.Up(ctx, db)
	if err != nil {
		_ = db.Close()
		return nil, 0, err
	}

	store, err := postgresstore.New(db)
	if err != nil {
		_ = db.Close()
		return nil, 0, err
	}
	return store, status.CurrentVersion, nil
}
