package bootstrap

import (
	"context"
	"fmt"

	"github.com/xsyetopz/go-mamacord/internal/config"
	migrate "github.com/xsyetopz/go-mamacord/internal/migration"
	postgresstore "github.com/xsyetopz/go-mamacord/internal/storage/postgres"
)

func OpenRuntimeStore(ctx context.Context, cfg config.Config) (*postgresstore.Store, int, error) {
	switch cfg.Storage.Backend {
	case "", config.StorageBackendPostgres:
		return openPostgresRuntimeStore(ctx, cfg)
	default:
		return nil, 0, fmt.Errorf("unsupported storage backend %q", cfg.Storage.Backend)
	}
}

func MigrationStatus(ctx context.Context, cfg config.Config) (migrate.Status, error) {
	runner, err := migrate.New(migrate.Options{
		Dir: cfg.Storage.Migrations,
	})
	if err != nil {
		return migrate.Status{}, err
	}

	switch cfg.Storage.Backend {
	case "", config.StorageBackendPostgres:
		db, err := postgresstore.Open(ctx, postgresstore.Options{DSN: cfg.Storage.PostgresDSN})
		if err != nil {
			return migrate.Status{}, err
		}
		defer db.Close()
		return runner.Status(ctx, db)
	default:
		return migrate.Status{}, fmt.Errorf("unsupported storage backend %q", cfg.Storage.Backend)
	}
}

func MigrateUp(ctx context.Context, cfg config.Config) (migrate.Status, error) {
	runner, err := migrate.New(migrate.Options{
		Dir: cfg.Storage.Migrations,
	})
	if err != nil {
		return migrate.Status{}, err
	}

	switch cfg.Storage.Backend {
	case "", config.StorageBackendPostgres:
		db, err := postgresstore.Open(ctx, postgresstore.Options{DSN: cfg.Storage.PostgresDSN})
		if err != nil {
			return migrate.Status{}, err
		}
		defer db.Close()
		return runner.Up(ctx, db)
	default:
		return migrate.Status{}, fmt.Errorf("unsupported storage backend %q", cfg.Storage.Backend)
	}
}

func openPostgresRuntimeStore(ctx context.Context, cfg config.Config) (*postgresstore.Store, int, error) {
	db, err := postgresstore.Open(ctx, postgresstore.Options{DSN: cfg.Storage.PostgresDSN})
	if err != nil {
		return nil, 0, err
	}

	runner, err := migrate.New(migrate.Options{
		Dir: cfg.Storage.Migrations,
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
