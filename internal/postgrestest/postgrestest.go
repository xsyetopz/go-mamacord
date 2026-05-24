package postgrestest

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	migrate "github.com/xsyetopz/go-mamacord/internal/migration"
	"github.com/xsyetopz/go-mamacord/internal/postgres"
)

var schemaCounter uint64

func OpenEmptyDB(t testing.TB) *sql.DB {
	t.Helper()

	db, err := postgres.Open(context.Background(), postgres.Options{DSN: OpenSchemaDSN(t)})
	if err != nil {
		t.Fatalf("postgres.Open(test schema): %v", err)
	}
	return db
}

func OpenSchemaDSN(t testing.TB) string {
	t.Helper()

	baseDSN := strings.TrimSpace(os.Getenv("MAMACORD_TEST_POSTGRES_DSN"))
	if baseDSN == "" {
		t.Skip("set MAMACORD_TEST_POSTGRES_DSN to run Postgres-backed tests")
	}

	ctx := context.Background()
	adminDB, err := postgres.Open(ctx, postgres.Options{DSN: baseDSN})
	if err != nil {
		t.Fatalf("postgres.Open(admin): %v", err)
	}

	schemaName := fmt.Sprintf("mamacord_test_%d", atomic.AddUint64(&schemaCounter, 1))
	if _, err := adminDB.ExecContext(ctx, "CREATE SCHEMA "+quoteIdent(schemaName)); err != nil {
		_ = adminDB.Close()
		t.Fatalf("create schema %q: %v", schemaName, err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.ExecContext(context.Background(), "DROP SCHEMA "+quoteIdent(schemaName)+" CASCADE")
		_ = adminDB.Close()
	})

	return dsnWithSearchPath(t, baseDSN, schemaName)
}

func OpenMigratedDB(t testing.TB) *sql.DB {
	t.Helper()

	db := OpenEmptyDB(t)
	runner, err := migrate.New(migrate.Options{
		Dir: filepath.Join(repoRoot(t), "migrations", "postgres"),
	})
	if err != nil {
		_ = db.Close()
		t.Fatalf("migrate.New: %v", err)
	}
	if _, err := runner.Up(context.Background(), db); err != nil {
		_ = db.Close()
		t.Fatalf("runner.Up: %v", err)
	}
	return db
}

func dsnWithSearchPath(t testing.TB, rawDSN string, searchPath string) string {
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

func quoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func repoRoot(t testing.TB) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
