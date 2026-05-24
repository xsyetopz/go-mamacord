package migrate_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	migrate "github.com/xsyetopz/go-mamacord/internal/migration"
	"github.com/xsyetopz/go-mamacord/internal/postgrestest"
)

func TestRunnerUpIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	writeMigrationUp(t, dir, 1, "init", migrate.KindNormal,
		"CREATE TABLE IF NOT EXISTS t1 (id BIGINT PRIMARY KEY);",
	)

	runner, db := newRunnerAndDB(t, dir)
	defer db.Close()

	status, err := runner.Up(ctx, db)
	if err != nil {
		t.Fatalf("Up(1): %v", err)
	}
	if status.CurrentVersion != 1 {
		t.Fatalf("unexpected current version after first up: %d", status.CurrentVersion)
	}

	status, err = runner.Up(ctx, db)
	if err != nil {
		t.Fatalf("Up(2): %v", err)
	}
	if status.CurrentVersion != 1 {
		t.Fatalf("unexpected current version after second up: %d", status.CurrentVersion)
	}
	if len(status.Applied) != 1 || len(status.Pending) != 0 {
		t.Fatalf("unexpected status: %#v", status)
	}
}

func TestRunnerRejectsChecksumMismatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	writeMigrationUp(t, dir, 1, "init", migrate.KindNormal,
		"CREATE TABLE IF NOT EXISTS t1 (id BIGINT PRIMARY KEY);",
	)

	runner, db := newRunnerAndDB(t, dir)
	defer db.Close()

	if _, err := runner.Up(ctx, db); err != nil {
		t.Fatalf("Up: %v", err)
	}

	writeMigrationUp(t, dir, 1, "init", migrate.KindNormal,
		"CREATE TABLE IF NOT EXISTS t1 (id BIGINT PRIMARY KEY, name TEXT NOT NULL DEFAULT 'x');",
	)

	if _, err := runner.Status(ctx, db); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestRunnerRejectsUnsupportedMigrationFilename(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	writeMigrationFile(t, filepath.Join(dir, "001_init.up.sql"), "-- migrate:kind=normal\nCREATE TABLE t1(id INTEGER PRIMARY KEY);")
	writeMigrationFile(t, filepath.Join(dir, "001_init.down.sql"), "DROP TABLE t1;")

	runner, db := newRunnerAndDB(t, dir)
	defer db.Close()

	if _, err := runner.Status(ctx, db); err == nil || !strings.Contains(err.Error(), "unsupported migration filename") {
		t.Fatalf("expected unsupported filename error, got %v", err)
	}
}

func TestProjectMigrationsExcludeLegacyGuildTables(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	migrationsDir := filepath.Join(repoRoot, "migrations", "postgres")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", migrationsDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".up.sql") {
			continue
		}
		bytes, err := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", entry.Name(), err)
		}
		text := string(bytes)
		if strings.Contains(text, "guild_plugins") {
			t.Fatalf("legacy guild_plugins table still present in %s", entry.Name())
		}
		if strings.Contains(text, "guild_settings") {
			t.Fatalf("legacy guild_settings table still present in %s", entry.Name())
		}
	}
}

func newRunnerAndDB(t *testing.T, dir string) (migrate.Runner, *sql.DB) {
	t.Helper()

	runner, err := migrate.New(migrate.Options{
		Dir: dir,
	})
	if err != nil {
		t.Fatalf("migrate.New: %v", err)
	}

	return runner, postgrestest.OpenEmptyDB(t)
}

func writeMigrationUp(t *testing.T, dir string, version int, name string, kind migrate.Kind, upSQL string) {
	t.Helper()

	upPath := filepath.Join(dir, formatMigrationFilename(version, name, "up"))

	writeMigrationFile(t, upPath, "-- migrate:kind="+string(kind)+"\n"+strings.TrimSpace(upSQL)+"\n")
}

func writeMigrationFile(t *testing.T, path, sqlText string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(sqlText), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func formatMigrationFilename(version int, name, direction string) string {
	return fmt.Sprintf("%03d_%s.%s.sql", version, name, direction)
}
