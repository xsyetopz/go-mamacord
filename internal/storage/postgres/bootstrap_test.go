package postgresstore_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/config"
	postgresstore "github.com/xsyetopz/go-mamacord/internal/storage/postgres"
)

func TestOpenRuntimeStorePostgresBackendIntegration(t *testing.T) {
	t.Parallel()

	baseDSN := strings.TrimSpace(os.Getenv("MAMACORD_TEST_POSTGRES_DSN"))
	if baseDSN == "" {
		t.Skip("set MAMACORD_TEST_POSTGRES_DSN to run live Postgres integration")
	}

	ctx := context.Background()
	adminDB, err := postgresstore.Open(ctx, postgresstore.Options{DSN: baseDSN})
	if err != nil {
		t.Fatalf("postgresstore.Open(admin): %v", err)
	}
	defer adminDB.Close()

	schemaName := fmt.Sprintf("mamacord_test_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, "CREATE SCHEMA "+postgresQuoteIdent(schemaName)); err != nil {
		t.Fatalf("create schema %q: %v", schemaName, err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.ExecContext(context.Background(), "DROP SCHEMA "+postgresQuoteIdent(schemaName)+" CASCADE")
	})

	cfg := config.Config{
		Storage: config.StorageConfig{
			Backend:     config.StorageBackendPostgres,
			PostgresDSN: postgresDSNWithSearchPath(t, baseDSN, schemaName),
			Migrations:  filepath.Clean(filepath.Join("..", "..", "migrations", "postgres")),
		},
	}

	store, version, err := postgresstore.OpenRuntimeStore(ctx, cfg)
	if err != nil {
		t.Fatalf("OpenRuntimeStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if version != 9 {
		t.Fatalf("unexpected migration version: %d", version)
	}

	status, err := postgresstore.MigrationStatus(ctx, cfg)
	if err != nil {
		t.Fatalf("MigrationStatus: %v", err)
	}
	if status.CurrentVersion != 9 {
		t.Fatalf("unexpected status version: %d", status.CurrentVersion)
	}
}

func postgresDSNWithSearchPath(t *testing.T, rawDSN string, searchPath string) string {
	t.Helper()

	parsed, err := url.Parse(strings.TrimSpace(rawDSN))
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", rawDSN, err)
	}
	query := parsed.Query()
	query.Set("search_path", strings.TrimSpace(searchPath))
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func postgresQuoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
