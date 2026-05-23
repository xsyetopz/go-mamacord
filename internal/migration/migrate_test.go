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
	"github.com/xsyetopz/go-mamacord/internal/sqlite"
)

func TestRunnerUpIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	writeMigrationUp(t, dir, 1, "init", migrate.KindNormal,
		"CREATE TABLE IF NOT EXISTS t1 (id INTEGER PRIMARY KEY);",
	)

	runner, dbPath := newRunnerAndPath(t, dir)
	status, err := runner.UpPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("UpPath(1): %v", err)
	}
	if status.CurrentVersion != 1 {
		t.Fatalf("unexpected current version after first up: %d", status.CurrentVersion)
	}

	status, err = runner.UpPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("UpPath(2): %v", err)
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
		"CREATE TABLE IF NOT EXISTS t1 (id INTEGER PRIMARY KEY);",
	)

	runner, dbPath := newRunnerAndPath(t, dir)
	if _, err := runner.UpPath(ctx, dbPath); err != nil {
		t.Fatalf("UpPath: %v", err)
	}

	writeMigrationUp(t, dir, 1, "init", migrate.KindNormal,
		"CREATE TABLE IF NOT EXISTS t1 (id INTEGER PRIMARY KEY, name TEXT NOT NULL DEFAULT 'x');",
	)

	if _, err := runner.StatusPath(ctx, dbPath); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestRunnerRejectsUnsupportedMigrationFilename(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	writeMigrationFile(t, filepath.Join(dir, "001_init.up.sql"), "-- migrate:kind=normal\nCREATE TABLE t1(id INTEGER PRIMARY KEY);")
	writeMigrationFile(t, filepath.Join(dir, "001_init.down.sql"), "DROP TABLE t1;")

	runner, dbPath := newRunnerAndPath(t, dir)
	if _, err := runner.StatusPath(ctx, dbPath); err == nil || !strings.Contains(err.Error(), "unsupported migration filename") {
		t.Fatalf("expected unsupported filename error, got %v", err)
	}
}

func TestProjectMigrationsExcludeLegacyGuildTables(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "project.sqlite")

	runner, err := migrate.New(migrate.Options{
		Dir:       filepath.Join(repoRoot, "migrations", "sqlite"),
		BackupDir: filepath.Join(dir, "migration_backups"),
	})
	if err != nil {
		t.Fatalf("migrate.New: %v", err)
	}
	status, err := runner.UpPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("UpPath: %v", err)
	}
	if status.CurrentVersion != 7 {
		t.Fatalf("unexpected current version: %d", status.CurrentVersion)
	}

	db, err := sqlite.Open(ctx, sqlite.Options{Path: dbPath})
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	defer db.Close()

	assertTableExists(t, ctx, db, "guild_plugins", false)
	assertTableExists(t, ctx, db, "guild_settings", false)
}

func TestRunnerBackupPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	writeMigrationUp(t, dir, 1, "init", migrate.KindNormal,
		"CREATE TABLE IF NOT EXISTS t1 (id INTEGER PRIMARY KEY);",
	)

	runner, dbPath := newRunnerAndPath(t, dir)
	if _, err := runner.UpPath(ctx, dbPath); err != nil {
		t.Fatalf("UpPath: %v", err)
	}

	backupPath, err := runner.BackupPath(ctx, dbPath)
	if err != nil {
		t.Fatalf("BackupPath: %v", err)
	}
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("Stat(%q): %v", backupPath, err)
	}
}

func newRunnerAndPath(t *testing.T, dir string) (migrate.Runner, string) {
	t.Helper()

	runner, err := migrate.New(migrate.Options{
		Dir:       dir,
		BackupDir: filepath.Join(dir, "migration_backups"),
	})
	if err != nil {
		t.Fatalf("migrate.New: %v", err)
	}

	return runner, filepath.Join(dir, "test.sqlite")
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

func assertTableExists(t *testing.T, ctx context.Context, db *sql.DB, tableName string, want bool) {
	t.Helper()

	var n int
	if err := db.QueryRowContext(
		ctx,
		"SELECT COUNT(1) FROM sqlite_master WHERE type='table' AND name = ?",
		tableName,
	).Scan(&n); err != nil {
		t.Fatalf("query sqlite_master for %s: %v", tableName, err)
	}
	if got := n == 1; got != want {
		t.Fatalf("unexpected table existence for %s: got %v want %v", tableName, got, want)
	}
}
