package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Kind string

const (
	KindNormal      Kind = "normal"
	KindDestructive Kind = "destructive"
)

type Options struct {
	Dir string
}

type Runner struct {
	dir string
}

type Migration struct {
	Version    int
	Name       string
	Kind       Kind
	UpFilename string
	UpSQL      string
	UpChecksum string
}

type AppliedMigration struct {
	Version   int
	Name      string
	Kind      Kind
	Filename  string
	Checksum  string
	AppliedAt time.Time
}

type Item struct {
	Version    int
	Name       string
	Kind       Kind
	Applied    bool
	AppliedAt  *time.Time
	UpFilename string
}

type Status struct {
	CurrentVersion int
	Applied        []Item
	Pending        []Item
}

var migrationNamePattern = regexp.MustCompile(`^(\d{3})_(.+)\.up\.sql$`)

func New(opts Options) (Runner, error) {
	dir := strings.TrimSpace(opts.Dir)
	if dir == "" {
		return Runner{}, errors.New("migrations dir is required")
	}

	return Runner{
		dir: dir,
	}, nil
}

func (r Runner) Up(ctx context.Context, db *sql.DB) (Status, error) {
	defs, err := r.loadDefinitions()
	if err != nil {
		return Status{}, err
	}
	if err := ensureMetadataTables(ctx, db); err != nil {
		return Status{}, err
	}

	applied, err := loadAppliedState(ctx, db)
	if err != nil {
		return Status{}, err
	}
	if err := validateAppliedChecksums(applied, defs); err != nil {
		return Status{}, err
	}

	for _, def := range defs {
		if _, ok := applied[def.Version]; ok {
			continue
		}
		if def.Kind == KindDestructive {
			return Status{}, fmt.Errorf("refusing to apply destructive migration %q; restore from backup for rollbacks instead", def.UpFilename)
		}
		if err := applyUp(ctx, db, def); err != nil {
			return Status{}, err
		}
		applied[def.Version] = AppliedMigration{
			Version:   def.Version,
			Name:      def.Name,
			Kind:      def.Kind,
			Filename:  def.UpFilename,
			Checksum:  def.UpChecksum,
			AppliedAt: time.Now().UTC(),
		}
	}

	return buildStatus(defs, applied), nil
}

func (r Runner) Status(ctx context.Context, db *sql.DB) (Status, error) {
	defs, err := r.loadDefinitions()
	if err != nil {
		return Status{}, err
	}
	if err := ensureMetadataTables(ctx, db); err != nil {
		return Status{}, err
	}

	applied, err := loadAppliedState(ctx, db)
	if err != nil {
		return Status{}, err
	}
	if err := validateAppliedChecksums(applied, defs); err != nil {
		return Status{}, err
	}

	return buildStatus(defs, applied), nil
}

func (r Runner) loadDefinitions() ([]Migration, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %q: %w", r.dir, err)
	}

	byVersion := map[int]string{}
	byVersionName := map[int]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".sql") {
			continue
		}

		matches := migrationNamePattern.FindStringSubmatch(name)
		if matches == nil {
			return nil, fmt.Errorf("unsupported migration filename %q", name)
		}

		version, err := strconv.Atoi(matches[1])
		if err != nil {
			return nil, fmt.Errorf("parse migration version %q: %w", matches[1], err)
		}
		if byVersion[version] != "" {
			return nil, fmt.Errorf("duplicate migration version %03d", version)
		}
		byVersion[version] = name
		byVersionName[version] = matches[2]
	}

	versions := make([]int, 0, len(byVersion))
	for version := range byVersion {
		versions = append(versions, version)
	}
	sort.Ints(versions)

	defs := make([]Migration, 0, len(versions))
	expectedVersion := 1
	for _, version := range versions {
		if version != expectedVersion {
			return nil, fmt.Errorf("migration versions must be contiguous: expected %03d, found %03d", expectedVersion, version)
		}
		expectedVersion++

		filename := byVersion[version]
		upSQL, upChecksum, err := readMigrationFile(filepath.Join(r.dir, filename))
		if err != nil {
			return nil, err
		}
		kind, err := parseKind(upSQL, filename)
		if err != nil {
			return nil, err
		}

		defs = append(defs, Migration{
			Version:    version,
			Name:       byVersionName[version],
			Kind:       kind,
			UpFilename: filename,
			UpSQL:      upSQL,
			UpChecksum: upChecksum,
		})
	}

	return defs, nil
}

func ensureMetadataTables(ctx context.Context, db *sql.DB) error {
	const stateTable = `
CREATE TABLE IF NOT EXISTS schema_migration_state (
	version INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	kind TEXT NOT NULL,
	filename TEXT NOT NULL,
	checksum TEXT NOT NULL,
	applied_at INTEGER NOT NULL
);`
	if _, err := db.ExecContext(ctx, stateTable); err != nil {
		return fmt.Errorf("create schema_migration_state: %w", err)
	}

	const historyTable = `
CREATE TABLE IF NOT EXISTS schema_migration_history (
	id BIGSERIAL PRIMARY KEY,
	version INTEGER NOT NULL,
	name TEXT NOT NULL,
	direction TEXT NOT NULL,
	kind TEXT NOT NULL,
	filename TEXT NOT NULL,
	checksum TEXT NOT NULL,
	applied_at INTEGER NOT NULL
);`
	if _, err := db.ExecContext(ctx, historyTable); err != nil {
		return fmt.Errorf("create schema_migration_history: %w", err)
	}

	return nil
}

func loadAppliedState(ctx context.Context, db *sql.DB) (map[int]AppliedMigration, error) {
	rows, err := db.QueryContext(
		ctx,
		`SELECT version, name, kind, filename, checksum, applied_at
		 FROM schema_migration_state
		 ORDER BY version ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query schema_migration_state: %w", err)
	}
	defer rows.Close()

	out := map[int]AppliedMigration{}
	for rows.Next() {
		var row AppliedMigration
		var kind string
		var appliedAt int64
		if err := rows.Scan(
			&row.Version,
			&row.Name,
			&kind,
			&row.Filename,
			&row.Checksum,
			&appliedAt,
		); err != nil {
			return nil, fmt.Errorf("scan schema_migration_state row: %w", err)
		}
		row.Kind = Kind(kind)
		row.AppliedAt = time.Unix(appliedAt, 0).UTC()
		out[row.Version] = row
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema_migration_state: %w", err)
	}

	return out, nil
}

func validateAppliedChecksums(applied map[int]AppliedMigration, defs []Migration) error {
	defByVersion := make(map[int]Migration, len(defs))
	for _, def := range defs {
		defByVersion[def.Version] = def
	}

	for version, state := range applied {
		def, ok := defByVersion[version]
		if !ok {
			return fmt.Errorf("applied migration version %03d is not present in the migration directory", version)
		}
		if state.Checksum != def.UpChecksum {
			return fmt.Errorf(
				"checksum mismatch for migration %03d_%s: applied %s current %s",
				version,
				def.Name,
				state.Checksum,
				def.UpChecksum,
			)
		}
	}

	return nil
}

func applyUp(ctx context.Context, db *sql.DB, def Migration) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx for migration %q: %w", def.UpFilename, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, def.UpSQL); err != nil {
		return fmt.Errorf("exec migration %q: %w", def.UpFilename, err)
	}

	now := time.Now().UTC().Unix()
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO schema_migration_state(version, name, kind, filename, checksum, applied_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		def.Version,
		def.Name,
		string(def.Kind),
		def.UpFilename,
		def.UpChecksum,
		now,
	); err != nil {
		return fmt.Errorf("record schema_migration_state for %q: %w", def.UpFilename, err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO schema_migration_history(version, name, direction, kind, filename, checksum, applied_at)
		 VALUES ($1, $2, 'up', $3, $4, $5, $6)`,
		def.Version,
		def.Name,
		string(def.Kind),
		def.UpFilename,
		def.UpChecksum,
		now,
	); err != nil {
		return fmt.Errorf("record schema_migration_history for %q: %w", def.UpFilename, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %q: %w", def.UpFilename, err)
	}

	return nil
}
func readMigrationFile(path string) (string, string, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read migration %q: %w", filepath.Base(path), err)
	}
	sqlText := strings.TrimSpace(string(bytes))
	if sqlText == "" {
		return "", "", fmt.Errorf("migration %q is empty", filepath.Base(path))
	}
	sum := sha256.Sum256(bytes)
	return sqlText, hex.EncodeToString(sum[:]), nil
}

func parseKind(sqlText, filename string) (Kind, error) {
	lines := strings.Split(sqlText, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, "--") {
			return "", fmt.Errorf("migration %q must begin with a kind header comment", filename)
		}
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "--"))
		if !strings.HasPrefix(trimmed, "migrate:kind=") {
			return "", fmt.Errorf("migration %q must begin with a kind header comment", filename)
		}
		switch Kind(strings.TrimSpace(strings.TrimPrefix(trimmed, "migrate:kind="))) {
		case KindNormal:
			return KindNormal, nil
		case KindDestructive:
			return KindDestructive, nil
		default:
			return "", fmt.Errorf("migration %q has unsupported kind header %q", filename, trimmed)
		}
	}
	return "", fmt.Errorf("migration %q must declare a kind header", filename)
}

func buildStatus(defs []Migration, applied map[int]AppliedMigration) Status {
	status := Status{
		CurrentVersion: highestAppliedVersion(applied),
		Applied:        make([]Item, 0, len(applied)),
		Pending:        make([]Item, 0, len(defs)),
	}

	for _, def := range defs {
		item := Item{
			Version:    def.Version,
			Name:       def.Name,
			Kind:       def.Kind,
			UpFilename: def.UpFilename,
		}
		if state, ok := applied[def.Version]; ok {
			item.Applied = true
			appliedAt := state.AppliedAt
			item.AppliedAt = &appliedAt
			status.Applied = append(status.Applied, item)
			continue
		}
		status.Pending = append(status.Pending, item)
	}

	return status
}

func highestAppliedVersion(applied map[int]AppliedMigration) int {
	maxVersion := 0
	for version := range applied {
		if version > maxVersion {
			maxVersion = version
		}
	}
	return maxVersion
}
